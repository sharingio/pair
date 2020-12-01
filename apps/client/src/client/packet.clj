(ns client.packet
  (:refer-clojure :exclude [load])
  (:require [cheshire.core :as json]
            [clojure.tools.logging :as log]
            [clj-http.client :as http]
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
    ; when box is first made, there won't be a transition time avialble.
    ; in this instant, return the current time
    (if (nil? last-transition-time)
      (time/format (time/instant))
      last-transition-time)))

(s/fdef k8stime->unix-timestamp
  :args (s/cat :k8s-time string?)
  :ret int?)
(defn k8stime->unix-timestamp
  "Takes timestamp like 2020-11-30T21:32:44Z, and returns it as seconds from epoch"
  [k8stime]
  (time/to-millis-from-epoch k8stime))

(defn pluralize
  [n s]
  (if (= 1 n)
    (str n" "s)
    (str n" "s"s")))

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
           (str (pluralize days"day")", "))
         (when(or (> days 0)(> hours 0))
           (str (pluralize hours"hour")", "))
         (pluralize minutes"minute"))))

(defn get-backend
  "wrapper for fetching from backend and timing out if response takes longer than 2 seconds"
  [endpoint]
  (try+
   (-> (http/get endpoint {:socket-timeout 2000 :connection-timeout 2000})
       :body (json/decode true))
   (catch Object _
     (log/warn (str "Call to " endpoint "took too long or other error"))
     nil)))

(defn fetch-instance
  "Fetch raw data for each of our main instance endpoints"
  [instance-id]
  (let [endpoint (str backend-address "/api/instance/kubernetes/"instance-id)]
    {:instance (get-backend endpoint)
     :kubeconfig (get-backend (str endpoint "/kubeconfig"))
     :tmate-ssh (get-backend (str endpoint "/tmate/ssh"))
     :tmate-web (get-backend (str endpoint "/tmate/web"))
     :ingresses (get-backend (str endpoint "/ingresses"))}))

(defn get-sites
  [ingresses]
  (let [items (-> ingresses  :spec :items)
        rules (mapcat #(map :host (-> % :spec :rules)) items)
        tls (mapcat #(mapcat :hosts (-> % :spec :tls)) items)]
    (map (fn [addr]
           (if (some #{addr} tls)
             (str "https://"addr)
             (str "http://"addr))) rules)))

(defn launch
  [{:keys [username token]} {:keys [project timezone envvars facility type guests fullname email repos] :as params}]
  (let [backend (str "http://"(env :backend-address)"/api/instance")
        instance-spec {:type type
                       :facility facility
                       :setup {:user username
                               :guests (if (empty? guests)
                                         [ ]
                                         (clojure.string/split guests #" "))
                               :githubOAuthToken token
                               :env (if (empty? envvars) [] (text->env-map envvars))
                               :timezone timezone
                               :repos (if (empty? repos)
                                        [ ]
                                        (clojure.string/split repos #" "))
                               :fullname fullname
                               :email email}}
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
     :status (str api-response": "phase)}))

(defn get-instance
  [instance-id]
  (let [{:keys [instance kubeconfig tmate-ssh tmate-web ingresses]} (fetch-instance instance-id)]
    {:instance-id (or (-> instance :spec :name) instance-id)
     :owner (-> instance :setup :user)
     :guests (-> instance :setup :guests)
     :repos (-> instance :setup :repos)
     :facility (-> instance :spec :facility)
     :type (-> instance :spec :type)
     :phase (-> instance :status :phase)
     :uid (-> instance :status :resources :MachineStatus :nodeRef :uid)
     :timezone (-> instance :setup :timezone)
     :kubeconfig (-> kubeconfig :spec)
     :tmate-ssh (-> tmate-ssh :spec)
     :tmate-web (-> tmate-web :spec)
     :ingresses (-> ingresses :spec)
     :sites (get-sites (-> ingresses :spec))
     :created-at (status->created-at (:status instance))
     :age (relative-age (status->created-at (:status instance)))
     :spec (:spec instance)}))

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

(comment
  (def ztel(fetch-instance "zachmandeville-ztel"))

  (:uid (get-instance "zachmandeville-ztel"))


  (def insts (-> (http/get (str backend-address"/api/instance/kubernetes"))
                                :body (json/decode true) :list))

 )
