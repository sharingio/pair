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
  (:use [slingshot.slingshot :only [throw+ try+]]))

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

(defn relative-age
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
         (when(and (> days 0)(> hours 0))
           (str (pluralize hours"hour")", "))
         (pluralize minutes"minute"))))

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
  (let [{:keys [spec status] :as instance}
        (try+ (-> (http/get (str backend-address"/api/instance/kubernetes/"instance-id))
                  :body (json/decode true))
              (catch Object _
                (log/error "no http response for instance " instance-id)))
        setup (:setup spec)
        created-at (status->created-at status)]
    {:instance-id (or (:name spec) instance-id)
     :owner (:user setup)
     :guests (:guests setup)
     :repos (:repos setup)
     :facility (:facility spec)
     :type (:type spec)
     :phase (:phase status)
     :timezone (:timezone setup)
     :created-at created-at
     :age (relative-age created-at)
     :spec spec}))

(defn get-phase
  [instance-id]
  (try+ (-> (http/get (str backend-address"/api/instance/kubernetes/"instance-id))
            :body
            (json/decode true) :status :phase)
        (catch Object _ ;; The first time it is pinged we sometimes get an HTTPNoResponse
          (log/error "no http response for instance " instance-id)
          "Not Ready")))

(defn get-kubeconfig
  [phase instance-id]
  (if (= "Provisioning" phase) nil
      (try+ (-> (http/get (str backend-address"/api/instance/kubernetes/"instance-id"/kubeconfig"))
                :body (json/decode true)
                :spec json/generate-string
                yaml/parse-string
                (yaml/generate-string :dumper-options {:flow-style :block}))
            (catch Object _
              (log/error "tried to get kubeconfig, no luck for " instance-id)))))

(defn get-tmate-ssh
  [kubeconfig instance_id]
  (if (nil? kubeconfig) "Not ready to fetch tmate session"
      (try+ (-> (http/get (str backend-address"/api/instance/kubernetes/"instance_id"/tmate/ssh"))
                :body (json/decode true) :spec)
            (catch Object _
              (log/error "tried to get tmate, no luck for " instance_id)
              nil))))

(defn get-tmate-web
  [kubeconfig instance-id]
  (if (nil? kubeconfig) "Not ready to fetch tmate session"
      (try+ (-> (http/get (str backend-address"/api/instance/kubernetes/"instance-id"/tmate/web"))
                :body (json/decode true) :spec)
            (catch Object _
              (log/error "tried to get tmate, no luck for " instance-id)
              "No Tmate session yet"))))

(defn get-ingresses
  [instance-id]
  (try+ (-> (http/get (str backend-address"/api/instance/kubernetes/"instance-id"/ingresses"))
            :body (json/decode true))
            (catch Object _
              (log/error "tried to get ingress, no luck for " instance-id)
              nil)))

(defn get-sites
  [ingresses]
  (let [items (-> ingresses  :spec :items)
        rules (mapcat #(map :host (-> % :spec :rules)) items)
        tls (mapcat #(mapcat :hosts (-> % :spec :tls)) items)]
    (map (fn [addr]
           (if (some #{addr} tls)
             (str "https://"addr)
             (str "http://"addr))) rules)))

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
