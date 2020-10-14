(ns syme.packet
  (:require [cheshire.core :as json]
            [clojure.java.io :as io]
            [clojure.java.jdbc :as sql]
            [clojure.tools.logging :as log]
            [clj-http.client :as http]
            [environ.core :refer [env]]
            [syme.db :as db])
  (:use [slingshot.slingshot :only [throw+ try+]]))

(def backend-address (str "http://"(env :backend-address)))

(defn launch
  [username {:keys [project facility type guests identity credential] :as params}]
  (println "launch fn params: " params)
  (println "launch fn descruct: " project facility type)
  (let [backend (str "http://"(env :backend-address)"/api/instance")
        instance-spec {:type type
                       :facility facility
                       :setup {:user username
                               :guests [ guests ]
                               :repos [ project ]}}
        response (-> (http/post backend {:form-params instance-spec :content-type :json})
                     (:body)
                     (json/decode true))
        {{api-response :response} :metadata
               {phase :phase} :status
               {name :name} :spec} response]
    (db/create {:owner username
                :project project
                :facility facility
                :type type
                :instance-id name
                :status (str api-response": "phase)})
        (println "response: " api-response phase name)))

(defn kubeconfig-available?
  "String->Boolean
  Checks if response to get kubeconfig of given ID returns a completely empty config"
  [instance_id]
  (let
      [backend (str "http://"(env :backend-address)"/api/instance/kubernetes/"instance_id"/kubeconfig")
       kubeconfig (-> (http/get backend)
                      (:body)
                      (json/decode true)
                      (:spec))
       not-empty? (complement empty?)]
    (not-empty? (filter #(not-empty? (second %)) kubeconfig))))


(defn get-tmate
  "retrieve config for instance as json string "
  [instance_id]
  (let
      [tmate-address (str backend-address"/api/instance/kubernetes/"instance_id"/tmate")
       tmate-command (-> (http/get tmate-address)
                      (:body)
                      (json/decode true)
                      (:spec))]
       tmate-command))

(defn tmate-available?
  "InstanceID -> Boolean
  Does the tmate command given by get-tmate start with ssh? if so, it's a valid tmate command and not an err message"
  [instance_id]
  false)
;;  (clojure.string/starts-with? (get-tmate instance_id) "ssh"))


(defn status-levels
  "Status->String
  returns 1 through 5 depending on phases of cluster and humacs"
  [cluster humacs kubeconfig? tmate?]
  (cond
    (= "Pending" cluster) "1"
    (and (= "Pending" cluster) kubeconfig?) "2"
    (and (= "Provisioned" cluster)(empty? humacs)) "3"
    (and (= "Provisioned" cluster)
         (= "Running" humacs)
         (= tmate? false)) "4"
    :else "5"))

(defn get-status
  "get relevant status of instance including its level and message from api"
  [{:keys [instance_id]}]
  (let [status-address (str backend-address "/api/instance/kubernetes/" instance_id)
        status-response (-> (http/get status-address)
                            (:body)
                            (json/decode true)
                            (:status)
                            (:resources))
        cluster-status (-> status-response :Cluster :phase)]
    {:level "1"
     :cluster cluster-status
     :humacs "not running"
     :tmate false}))

        ;; cluster-status (-> status-response :Cluster :phase)
        ;; humacs-status (->  status-response :HumacsPod :phase)]
    ;; {:level (status-levels cluster-status humacs-status (kubeconfig-available? instance_id) (tmate-available? instance_id))
    ;;  :cluster cluster-status
    ;;  :humacs humacs-status
    ;;  :tmate (get-tmate instance_id)}))

(defn get-kubeconfig
  "retrieve config for instance as json string "
  [{:keys [instance_id]}]
  (let
      [backend (str "http://"(env :backend-address)"/api/instance/kubernetes/"instance_id"/kubeconfig")
       kubeconfig (-> (http/get backend)
                      (:body)
                      (json/decode true)
                      (:spec))]
  (json/generate-string kubeconfig)))


(let [status-address (str backend-address "/api/instance/kubernetes/" "zachmandeville-5e36941b3d-58bf3ee127")]
  (-> (http/get status-address)
      (:body)
      (json/decode true)))
