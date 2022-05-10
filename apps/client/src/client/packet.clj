(ns client.packet
  (:refer-clojure :exclude [load])
  (:require [cheshire.core :as json]
            [clojure.tools.logging :as log]
            [clj-http.client :as http]
            [org.httpkit.client :as client]
            [clojure.spec.alpha :as s]
            [java-time :as time]
            [clojure.string :as str]
            [clojure.spec.test.alpha :as test]
            [yaml.core :as yaml]
            [environ.core :refer [env]])
  (:use [slingshot.slingshot :only [throw+ try+]])
  (:import [java.util.concurrent TimeoutException TimeUnit]))

(def backend-address (str "http://"(env :backend-address)))

(s/fdef text->env
  :args (s/cat :text string?)
  :ret map?)
(defn text->env-map
  "given envvars on newlines
  separate key and value into {:key 'value'}"
  [text]
  (map (comp #(conj {} %) #(str/split % #"="))
       (str/split-lines text)))

(s/fdef status->created-at
  :args (s/cat :created-at map?)
  :ret any?); a java zoned-date-time, TODO correct predicate!
(defn status->created-at
  "Given  a cluster status map and timezone, return time its machine started as java local time"
  [status]
  (let [last-transition-time
        (->> status :resources :MachineStatus :conditions
             (filter #(= "Ready" (:type %))) first :lastTransitionTime)]
    last-transition-time))

(defn status->external-ips
  [status]
  (let [ips
        (->> status :resources :MachineStatus :addresses
             (filter #(= "ExternalIP" (:type %))))]
    (println "ips:" ips)
    ips))

(s/fdef k8stime->unix-timestamp
  :args (s/cat :k8s-time string?)
  :ret int?)
(defn k8stime->unix-timestamp
  "Takes timestamp like 2020-11-30T21:32:44Z, and returns it as seconds from epoch"
  [k8stime]
  (time/to-millis-from-epoch k8stime))

(defn pluralize
  [num unit]
  (str num" "unit(if(= 1 num)"""s")))

(defn relative-age
  "return string of hours,minutes,seconds difference between k8stime and now."
  [k8stime]
  (let [age-in-seconds
        (quot (-
               (time/to-millis-from-epoch (time/instant))
               (k8stime->unix-timestamp k8stime)) 1000)
        days (quot age-in-seconds 86400)
        hours (quot (mod age-in-seconds 86400) 3600)
        minutes (quot (mod (mod age-in-seconds 86400) 3600) 60)]
    (str (when (> days 0)
           (str (pluralize days "day") ", "))
         (when (or (> days 0) (> hours 0))
           (str (pluralize hours "hour") ", "))
         (pluralize minutes "minute"))))

(defn fetch-from-backend
  [url]
  (client/get url {:timeout 7000}
              (fn [{:keys [status headers body error]}] ;; asynchronous response handling
                (if error
                  (do (println "Failed, exception is " error) nil)
                  (json/decode body true)))))

(defn fetch-instance
  "Fetch raw data for each of our main instance endpoints"
  [instance-id]
  (let [endpoint (str backend-address "/api/instance/kubernetes/"instance-id)
        urls [[:instance endpoint]
              [:kubeconfig (str endpoint "/kubeconfig")]
              [:tmate-ssh (str endpoint "/tmate/ssh")]
              [:tmate-web (str endpoint "/tmate/web")]
              [:ingresses (str endpoint "/ingresses")]]
        futures (doall (map (fn [[name url]] [name (fetch-from-backend url)]) urls))
        results (doall (map (fn [[name future]] [name (deref future)]) futures))]
    (into {} results)))

(defn launch
  [{:keys [username token emails]} {:keys [name project timezone envvars facility type guests fullname email repos kubernetesNodeCount noGitHubToken] :as params}]
  (let [backend (str "http://"(env :backend-address)"/api/instance")
        instance-spec {:type type
                       :facility facility
                       :name name
                       :kubernetesNodeCount (Integer. (or kubernetesNodeCount 0))
                       :setup {:user username
                               :guests (if (empty? guests)
                                         [ ]
                                         (clojure.string/split guests #" "))
                               :githubOAuthToken (if (empty? noGitHubToken) "" token)
                               :env (if (empty? envvars) [] (text->env-map envvars))
                               :timezone timezone
                               :repos (if (empty? repos)
                                        [ ]
                                        (clojure.string/split repos #" "))
                               :fullname fullname
                               :email email
                               :extraEmails emails}}
        response (-> (http/post backend {:form-params instance-spec :content-type :json})
                     (:body)
                     (json/decode true))
        {{api-response :response} :metadata
         {phase :phase} :status
         {name :name} :spec} response]
    {:owner username
     :facility facility
     :type type
     :tmate-ssh nil
     :tmate-web nil
     :kubeconfig nil
     :guests guests
     :instance-id name
     :name name
     :timezone timezone
     :kubernetesNodeCount kubernetesNodeCount
     :status (str api-response": "phase)}))

(defn get-instance
  [instance-id]
  (let [{:keys [instance kubeconfig tmate-ssh tmate-web ingresses dns cert]} (fetch-instance instance-id)
        created-at (status->created-at (:status instance))]
    {:instance-id (or (-> instance :spec :name) instance-id)
     :owner (-> instance :spec :setup :user)
     :guests (-> instance :spec :setup :guests)
     :repos (-> instance :spec :setup :repos)
     :envvars (-> instance :spec :setup :env)
     :github-token (-> instance :spec :setup :githubOAuthToken)
     :facility (-> instance :spec :facility)
     :type (-> instance :spec :type)
     :phase (-> instance :status :phase)
     :nodeSize (-> instance :spec :nodeSize)
     :kubernetesNodeCount (-> instance :spec :kubernetesNodeCount)
     :uid (-> instance :status :resources :PacketMachineUID)
     :timezone (-> instance :spec :setup :timezone)
     :kubeconfig (-> kubeconfig :spec)
     :tmate-ssh (-> tmate-ssh :spec)
     :tmate-web (-> tmate-web :spec)
     :ingresses (map :url (:list ingresses))
     :sites (map :url (:list ingresses))
     :created-at created-at
     :external-ips (status->external-ips (:status instance))
     :age (if (nil? created-at) nil (relative-age created-at))}))

(defn get-all-instances
  [{:keys [username admin-member]}]
  (let [raw-instances (try+ (-> (http/get (str backend-address"/api/instance/kubernetes"))
                                :body (json/decode true) :list)
                            (catch Object _
                              (log/warn "Couldn't get instances")
                              []))
        instances (map (fn [{:keys [spec status]}]
                         {:instance-id (:name spec)
                          :phase (:phase status )
                          :created-at (status->created-at status)
                          :age (if (nil? (status->created-at status)) nil (relative-age (status->created-at status)))
                          :owner (-> spec :setup :user)
                          :guests (-> spec :setup :guests)
                          :repos (-> spec :setup :repos)
                         }) raw-instances)]
  (if admin-member
    instances
    (filter #(or (some #{username} (:guests %))
                 (= (:owner %) username)) instances))))

(defn delete-instance
  [instance-id]
  (http/delete (str backend-address"/api/instance/kubernetes/"instance-id)))
