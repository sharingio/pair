(ns syme.packet
  (:require [clojure.java.io :as io]
            [clojure.java.jdbc :as sql]
            [clojure.tools.logging :as log]
            [clj-http.client :as http]
            [environ.core :refer [env]]
            [syme.db :as db])
  (:use [slingshot.slingshot :only [throw+ try+]]))

(defn launch
  [username {:keys [project facility type guests identity credential]}]
  ;; TODO insert into db
  ;; post to backend
  (let [backend (str "http://"(env :backend-address)"/api/instance")
        instance-spec {:type type
                       :facility facility
                       :setup {:user username
                               :guests [ guests ]
                               :repos [ project ]}}
        response (http/post backend {:form-params instance-spec
                                     :content-type json})]
    (println reponse)))
