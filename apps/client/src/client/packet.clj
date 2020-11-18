(ns client.packet
  (:require [cheshire.core :as json]
            [next.jdbc :as jdbc]
            [next.jdbc.result-set :as rs]
            [clojure.tools.logging :as log]
            [clj-http.client :as http]
            [environ.core :refer [env]]
            [client.db :as db])
  (:use [slingshot.slingshot :only [throw+ try+]]))

(def backend-address (str "http://"(env :backend-address)))

(defn launch
  [username {:keys [project facility type guests fullname email repos] :as params}]
  (let [backend (str "http://"(env :backend-address)"/api/instance")
        instance-spec {:type type
                       :facility facility
                       :setup {:user username
                               :guests (if (empty? guests)
                                         [ ]
                                         (clojure.string/split guests #" "))
                               :repos (if (empty? repos)
                                        [ project ]
                                        (cons project (clojure.string/split repos #" ")))
                               :fullname fullname
                               :email email}}
        response (-> (http/post backend {:form-params instance-spec :content-type :json})
                     (:body)
                     (json/decode true))
        {{api-response :response} :metadata
               {phase :phase} :status
               {name :name} :spec} response]
    {:owner username
    :project project
    :facility facility
    :type type
    :guests guests
    :instance-id name
    :status (str api-response": "phase)}))
