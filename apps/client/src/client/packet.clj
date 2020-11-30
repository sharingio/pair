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

(s/fdef instance-age
  :args (s/cat :created-at map? :timezone string?)
  :ret any?); a java zoned-date-time, TODO correct predicate!
(defn status->created-at
  "Given  a cluster status map and timezone, return time its machine started as java local time"
  [status timezone]
  (let [last-transition-time (->> status :resources :MachineStatus :conditions
       (filter #(= "Ready" (:type %))) first :lastTransitionTime)]
    (-> last-transition-time time/zoned-date-time (time/with-zone timezone))))

(s/fdef instance-age
  :args (s/cat :created-at map? :timezone string?)
  :ret string?)
(defn instance-age
  "compares creation time of instnace to current local time and calculates age in hours or days."
  [created-at timezone]
  (let [now (time/with-zone (time/zoned-date-time) timezone)
        age-hours (time/time-between created-at now :hours)
        age-days (time/time-between created-at now :days)]
    (println "TIMENOW" now)
    (println "TIMETHEN" created-at)
    (cond
      (= 0 age-hours) "Less than an hour old"
      (< age-hours 24) (str age-hours" "(if (= 1 age-hours)"hour" "hours")" old")
      :else (str age-days" "(if (= 1 age-days)"day""days")" old"))))


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
        created-at (status->created-at status (:timezone setup))]
    {:instance-id (or (:name spec) instance-id)
     :owner (:user setup)
     :guests (:guests setup)
     :repos (:repos setup)
     :facility (:facility spec)
     :type (:type spec)
     :phase (:phase status)
     :timezone (:timezone setup)
     :created-at created-at
     :age (instance-age created-at (:timezone setup))
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

(comment
(let [items (-> (get-ingresses "hh-0iew") :spec :items)
      rules (mapcat #(map :host (-> % :spec :rules)) items)
      tls (mapcat #(mapcat :hosts (-> % :spec :tls)) items)]
  (map (fn [addr]
         (if (some #{addr} tls)
          (str "https://"addr)
          (str "http://"addr))) rules))

  )
