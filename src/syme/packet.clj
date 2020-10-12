(ns syme.packet
  (:require [cheshire.core :as json]
            [clojure.java.io :as io]
            [clojure.java.jdbc :as sql]
            [clojure.tools.logging :as log]
            [clj-http.client :as http]
            [environ.core :refer [env]]
            [syme.db :as db])
  (:use [slingshot.slingshot :only [throw+ try+]]))



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


