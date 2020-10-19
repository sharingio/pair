(ns syme.packet
  (:require [cheshire.core :as json]
            [clojure.java.io :as io]
            [clojure.java.jdbc :as sql]
            [tentacles.users :as users]
            [clojure.tools.logging :as log]
            [clj-http.client :as http]
            [environ.core :refer [env]]
            [syme.db :as db])
  (:use [slingshot.slingshot :only [throw+ try+]]))

(def backend-address (str "http://"(env :backend-address)))
(defonce email (memoize (comp :email users/user)))
(defonce github-name (memoize (comp :name users/user)))

(defn launch
  [username {:keys [project facility type guests identity credential] :as params}]
  (let [backend (str "http://"(env :backend-address)"/api/instance")
        instance-spec {:type type
                       :facility facility
                       :setup {:user username
                               :guests [ guests ]
                               :repos [ project ]
                               :fullname "Zach Mandeville DOG"
                               :email "zz@ii.coop"}}
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
                :status (str api-response": "phase)})))


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
  (clojure.string/starts-with? (get-tmate instance_id) "ssh"))


(defn status-levels
  "Status->String
  returns 1 through 5 depending on phases of cluster and humacs"
  [phase cluster humacs instance_id]
  (cond
    (= "Pending" phase) 1
    (= "Provisioning" phase) 2
    (and (= "Provisioned" phase)
         (empty? humacs)) 3
    (and (= "Provisioned" phase)
         (= "Running" humacs)
         (= (tmate-available? instance_id) false)) 4
    :else 5))

(defn get-status-response
  "Grab a status payload rom backend, or placeholder if not status available yet"
  [backend]
  (let [response (try+ (http/get backend) (catch Object _ "404"))]
    (if (= response "404")
      {:Cluster {:phase "Not available yet"}
       :HumacsPod {:phase "Not available yet"}}
      (-> response
          (:body)
          (json/decode true)
          (:status)))))

(defn get-status
  "get relevant status of instance including its level and message from api"
  [{:keys [instance_id]}]
  (let [
        status-address (str backend-address "/api/instance/kubernetes/" instance_id)
        status-response (get-status-response status-address)
        phase (-> status-response :phase)
        cluster-status (-> status-response :resources :Cluster :phase)
        humacs-status (-> status-response :resources :HumacsPod :phase)]
    {:level (status-levels phase cluster-status humacs-status instance_id)
     :phase phase
     :cluster cluster-status
     :humacs humacs-status
     :instance instance_id}))

(defn get-kubeconfig
  "retrieve config for instance as json string "
  [{:keys [instance_id]}]
  (let
      [backend (str "http://"(env :backend-address)"/api/instance/kubernetes/"instance_id"/kubeconfig")
       kubeconfig (-> (http/get backend)
                      (:body)
                      (json/decode true)
                      (:spec))]
    kubeconfig))

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
    (not-empty? kubeconfig)))
