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

(defn instance-phase
  [instance-id]
  (let [phase (try+ (-> (http/get (str backend-address"/api/instance/kubernetes/"instance-id))
                         :body
                         (json/decode true) :status :phase)
                     (catch Object _ ;; The first time it is pinged we sometimes get an HTTPNoResponse
                       (log/error "no http response for instance " instance-id)
                       "Not Ready"))]
    {:instance-id instance-id
     :phase phase}))

;; #+RESULTS: get all names of Kubernetes instances
;; #+begin_example
;; zachmandeville-kv74
;; zachmandeville-vc33
;; #+end_example

;; #+NAME: get a Kubernetes instance
;; #+begin_src shell
;; curl -X GET http://localhost:8080/api/instance/kubernetes/bobymcbobs-b556f7da7a-1a3866b444 | jq .
;; #+end_src

;; #+NAME: get tmate session for Kubernetes instance
;; #+begin_src shell
;; curl -X GET http://localhost:8080/api/instance/kubernetes/bobymcbobs-b556f7da7a-1a3866b444/tmate | jq .
;; #+end_src

;; #+NAME: get kubeconfig for Kubernetes instance
;; #+begin_src shell
;; curl -X GET http://localhost:8080/api/instance/kubernetes/bobymcbobs-b556f7da7a-128d9375a4/kubeconfig | jq .spec
;; #+end_src

;; #+NAME: get a list of all Kubernetes instances
;; #+begin_src shell
;; curl -X GET http://localhost:8080/api/instance/kubernetes | jq .
;; #+end_src

backend-address



(try+ (http/get (str backend-address "/api/instance/kubernetes/zachmandeville-kv76"))
      (catch Object _
        {:phase "Not Ready"
         :instance-id "instnace"}))
