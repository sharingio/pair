(ns client.web
  (:require [cheshire.core :as json]
            [clj-http.client :as http]
            [compojure.core :refer :all]
            [environ.core :refer [env]]
            [compojure.route :as route]
            [ring.middleware.session.cookie :as cookie]
            [ring.util.response :as res]
            [compojure.handler :refer [site]]
            [client.views :as views]
            [client.db :as db]
            [client.github :as gh]
            [client.packet :as packet]
            [ring.middleware.defaults :refer [wrap-defaults site-defaults]]))

(defroutes app-routes
  (GET "/" {session :session}
       (views/splash (:username session)))

  (GET "/new" {{:keys [username user] :as session} :session}
       (if username
         (views/new user)
         (res/redirect views/login-url)))

  (POST "/new" {{:keys [user] :as session} :session
                {:keys [project] :as params} :params}
        (when-not (:username user)
          (throw (ex-info "must be logged in" {:status 400})))
        (let [{:keys [instance-id] :as instance} (packet/launch user params)]
          (println "INSTANCE BYA!" instance-id)
          (assoc (res/redirect (str "instances/id/"instance-id))
                 :session (merge session {:instance instance}))))

  (GET "/instances" {{:keys [user instances]} :session}
       (views/all-instances instances user))

  (GET "/instances/id/:id" {{:keys [user instance]} :session}
       (views/instance instance (:username user)))

  (GET "/instances/id/:id/delete" {{:keys [user instance]} :session}
       (views/delete-instance instance (:username user)))

  (POST "/instances/id/:id/delete" {{:keys [user instance]} :session
                                    {:keys [instance-id]} :params}
        (packet/delete-instance instance-id)
        (res/redirect "/instances"))

  (GET "/logout" []
       (assoc (res/redirect "/") :session nil))

  (GET "/oauth"[code :as {session :session}]
       (if code
         (let [token (gh/get-token code)
               {username :login
                fullname :name
                avatar :avatar_url
                :as user}(gh/github-get "user" token)
               email (gh/get-primary-email token)
               permitted-org-member (gh/in-permitted-org? token)
               user {:username username
                     :fullname fullname
                     :avatar avatar
                     :email email
                     :permitted-org-member permitted-org-member}]
           (if (db/find-user username)
             (db/update-user user)
             (db/add-user user))
           (assoc (res/redirect "/")
                  :session (merge session {:token token :username username :user user})))))

  (route/not-found "Not Found"))

(defn wrap-get-all-instances
  [handler]
  (fn [req]
    (handler
     (if (= "/instances" (:uri req))
       (let [instances (packet/get-all-instances (-> req :session :user :username))]
         (assoc-in req [:session :instances] instances))
       (do (println "No Instances Found" (-> req :session)) req)))))

(defn wrap-update-instance
  [handler]
  (fn [req]
    (handler (if-let [instance-id (second (re-find #"/instances/id/([a-zA-Z0-9-]*)"
                                               (:uri req)))]
               (let [instance (packet/get-instance instance-id)
                     kubeconfig (packet/get-kubeconfig (:phase instance) instance-id)
                     tmate-ssh (packet/get-tmate-ssh kubeconfig instance-id)
                     tmate-web (packet/get-tmate-web kubeconfig instance-id)
                     status (merge instance {:kubeconfig kubeconfig
                                             :tmate-ssh tmate-ssh
                                             :tmate-web tmate-web})]
               (assoc-in req [:session :instance] (merge (-> req :session :instance) status)))
                  req ))))

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
        (wrap-get-all-instances)
        (wrap-update-instance)
        (wrap-logging)
        (wrap-defaults (assoc site-defaults :session {:store store})))))
