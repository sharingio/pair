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

(defn get-status
  "get relevant status of instance including its level and message from api"
  [{:keys [instance_id]}]
  (let [status-address (str backend-address "/api/instance/kubernetes/" instance_id)
        status-response (-> (http/get status-address)
                            (:body)
                            (json/decode true))]
    status-response))



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


;; (def instance {:instance_id "zachmandeville-5e36941b3d-65fd2a11ef"})
;; (get-status instance)
