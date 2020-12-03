(ns client.web
  (:require [cheshire.core :as json]
            [clj-http.client :as http]
            [compojure.core :refer :all]
            [environ.core :refer [env]]
            [compojure.route :as route]
            [ring.middleware.session.cookie :as cookie]
            [ring.util.response :as res]
            [ring.adapter.jetty :refer [run-jetty]]
            [compojure.handler :refer [site]]
            [client.views :as views]
            [client.github :as gh]
            [client.packet :as packet]
            [ring.middleware.defaults :refer [wrap-defaults site-defaults]])
  (:import [org.eclipse.jetty.server.handler.gzip GzipHandler])
  (:gen-class))

(defroutes app-routes
  (GET "/" {{:keys [user]} :session}
       (views/splash user))

  (GET "/instances" {{:keys [user instances]} :session}
       (views/all-instances instances user))

  (GET "/instances/new" {{:keys [username user] :as session} :session}
       (if (:username user)
         (views/new user)
         (res/redirect views/login-url)))

  (POST "/instances/new" {{:keys [user] :as session} :session
                          {:keys [project] :as params} :params}
        (when-not (:username user)
          (throw (ex-info "must be logged in" {:status 400})))
        (let [{:keys [instance-id] :as instance} (packet/launch user params)]
          (assoc (res/redirect (str "/instances/id/"instance-id))
                 :session (merge session {:instance instance}))))

  (GET "/instances/id/:id" {uri :uri
                            {:keys [user instance] :as session} :session}

       (let [{:keys [username admin-member]} user
             {:keys [owner guests]} instance
             owner-or-guest (some #{username} (conj guests owner))]
         (if (or admin-member owner-or-guest)
           (views/instance instance user)
           (res/redirect "/instances"))))

  (GET "/instances/id/:id/delete" {{:keys [user instance]} :session}
       (views/delete-instance instance user))

  (POST "/instances/id/:id/delete" {{:keys [user instance]} :session
                                    {:keys [instance-id]} :params}
        (packet/delete-instance instance-id)
        (res/redirect "/instances"))

  (GET "/public-instances/:uid/:instance-id" {{:keys [uid instance-id]} :params}
       (let [instance (packet/get-instance instance-id)]
         (if (and (:uid instance)(= uid (:uid instance)))
           (views/instance instance {:username "guest"})
           (assoc (res/redirect "/") :session nil))))

  (GET "/public-instances/:uid/:instance-id/kubeconfig" {{:keys [uid instance-id]} :params}
       (let [instance (packet/get-instance instance-id)]
         (if (and (:uid instance)(= uid (:uid instance)))
           {:status 200 :body (:kubeconfig instance)}
           (assoc (res/redirect "/") :session nil))))

  (GET "/logout" []
       (assoc (res/redirect "/") :session nil))

  (GET "/oauth"[code :as {session :session}]
       (if code
         (assoc (res/redirect "/")
                :session (merge session {:user (gh/get-user-info code)}))))

  (route/not-found "Not Found"))

(defn wrap-get-all-instances
  [handler]
  (fn [req]
    (handler
     (if (= "/instances" (:uri req))
       (let [instances (packet/get-all-instances (-> req :session :user))]
         (assoc req :session (merge (:session req) {:instances instances})))
       req))))

(defn wrap-update-instance
  [handler]
  (fn [req]
    (if-let [instance-id (second (re-find #"/instances/id/([a-zA-Z0-9-]*)" (:uri req)))]
       (let [instance (packet/get-instance instance-id)
             non-nil-instance (select-keys instance (for [[k v] instance :when (not (nil? v))] k))
             response (handler (assoc-in req [:session :instance] instance))]
         response)
      (handler req))))

(defn wrap-login
  [handler]
  (fn [req]
    (println "LOGIN" (-> req :session keys))
    (handler
    (if (or (#{"/" "/about" "/faq" "/404" "/oauth" "/logout"} (:uri req))
            (re-find #"/public-instances/([A-Za-z0-9-]*)/([A-Za-z0-9-])" (:uri req))
            (-> req :session :user :permitted-member))
      req
      {:status 401 :body "You Must be Logged in"}))))

(defn wrap-logging
  [handler]
  (fn [req]
    (let [{req :request-method
           proto :protocol
           headers :headers} req]
      ;; (println "\n req:"req"\n protocol: "proto "\n headers: " headers)
      )
    (handler req)))


(defn- add-gzip-handler [server]
  (.setHandler server
               (doto (GzipHandler.)
                 (.setIncludedMimeTypes (into-array ["text/css"
                                                     "text/plain"
                                                     "text/javascript"
                                                     "application/javascript"
                                                     "application/json"
                                                     "image/svg+xml"]))
                 (.setMinGzipSize 1024)
                 (.setHandler (.getHandler server)))))

(def app
  (let [store (cookie/cookie-store {:key (env :session-secret)})]
    (-> app-routes
        ;; wrap-logging
        wrap-get-all-instances
        wrap-update-instance
        wrap-login
        (wrap-defaults (assoc site-defaults :session {:store store})))))

(defn -main [& args]
  (run-jetty
   app {
        :port (Integer/valueOf (or (System/getenv "PORT") "5000"))
        :join? false
        :configurator add-gzip-handler}))
