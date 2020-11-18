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

  (GET "/launch" {{:keys [username] :as session} :session
                  {:keys [project] :as params} :params}
       (if-let [instance (db/find-instance username project)]
         (res/redirect (str "project/"project))
         (if username
           (views/launch username project)
           (assoc (res/redirect views/login-url)
                  :session (merge session {:project project})))))

  (POST "/launch" {{:keys [username] :as session} :session
                   {:keys [project] :as params} :params}
        (when-not username
          (throw (ex-info "must be logged in" {:status 400})))
        (when (db/find-instance username project)
          (throw (ex-info "already launched" {:status 403})))
        (db/new-instance (packet/launch username params))
        (assoc (res/redirect (str "project/"project)) :session (merge session {:project project})))


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
           (assoc (res/redirect (if (:project session) "/launch" "/"))
                  :session (merge session {:token token :username username})))))

  (GET "/project/:gh-user/:project" {{:keys [username]} :session
                                     instance :instance}
       (views/project username instance))


  (route/not-found "Not Found"))

(defn wrap-find-instance
  [handler]
  (fn [req]
    (handler (if-let [project (second (re-find #"/project/([^/]+/[^/]+)"
                                               (:uri req)))]
               (if-let [inst (db/find-instance (:username (:session req)) project)]
                 (assoc req :instance inst)
                 (throw (ex-info "Instance not found." {:status 404})))
               req))))

;; (println request-method uri protocol headers remote-addr))
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
        (wrap-find-instance)
        (wrap-logging)
        (wrap-defaults (assoc site-defaults :session {:store store})))))
