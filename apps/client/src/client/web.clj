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

  (GET "/instances/id/:id" {{{:keys [username admin-member] :as user} :user
                            {:keys [owner guests] :as instance} :instance} :session}
       (let [owner-or-guest (some #{username} (conj guests owner))]
         (if (or admin-member owner-or-guest)
           (views/instance instance user)
           (res/redirect "/instances"))))

  (GET "/instances/id/:id/delete" {{:keys [user instance]} :session}
       (views/delete-instance instance user))

  (POST "/instances/id/:id/delete" {{:keys [user instance]} :session
                                    {:keys [instance-id]} :params}
        (packet/delete-instance instance-id)
        (res/redirect "/instances"))

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
         (assoc-in req [:session :instances] instances))
       (do (println "No Instances Found" (-> req :session)) req)))))

(defn wrap-update-instance
  [handler]
  (fn [req]
    (handler (if-let [instance-id (second (re-find #"/instances/id/([a-zA-Z0-9-]*)"
                                               (:uri req)))]
               (let [instance (packet/get-instance instance-id)
                     kubeconfig nil;(packet/get-kubeconfig (:phase instance) instance-id)
                     tmate-ssh nil ;(packet/get-tmate-ssh kubeconfig instance-id)
                     tmate-web nil ;(packet/get-tmate-web kubeconfig instance-id)
                     ingresses nil ;(packet/get-ingresses instance-id)
                     sites (packet/get-sites ingresses)
                     status (merge instance {:kubeconfig kubeconfig
                                             :tmate-ssh tmate-ssh
                                             :tmate-web tmate-web
                                             :ingresses ingresses
                                             :sites sites})]
               (assoc-in req [:session :instance] (merge (-> req :session :instance) status)))
                  req ))))

(defn wrap-login
  [handler]
  (fn [req]
    (if (or (#{"/" "/about" "/faq" "/oauth" "/logout"} (:uri req))
            (-> req :session :user :permitted-member))
      (handler req)
      {:status 401 :body "You Must be Logged in"})))

(defn wrap-logging
  [handler]
  (fn [req]
    (let [{req :request-method
           proto :protocol
           headers :headers} req]
      (println "\n req:"req"\n protocol: "proto "\n headers: " headers))
    (handler req)))

(def app
  (let [store (cookie/cookie-store {:key (env :session-secret)})]
    (-> app-routes
        (wrap-login)
        (wrap-get-all-instances)
        (wrap-update-instance)
        (wrap-logging)
        (wrap-defaults (assoc site-defaults :session {:store store})))))

(defn -main [& args]
  (run-jetty app {:port (Integer/valueOf (or (System/getenv "PORT") "5000"))}))
