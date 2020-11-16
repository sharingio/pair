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
            [ring.middleware.defaults :refer [wrap-defaults site-defaults]]))

(defn get-token [code]
  (-> (http/post "https://github.com/login/oauth/access_token"
                 {:form-params {:client_id (env :oauth-client-id)
                                :client_secret (env :oauth-client-secret)
                                :code code}
                  :headers {"Accept" "application/json"}})
      :body (json/decode true) :access_token))

(defn github-get
  [endpoint token]
  (-> (http/get (str "https://api.github.com/" endpoint)
                {:headers {"accept" "application/json"
                           "Authorization" (str "token " token)}})
      :body (json/decode true)))

(defn get-primary-email
  [token]
  (let [emails (github-get "user/emails" token)]
    (:email (first (filter #(= (:primary %)true) emails)))))

(defn in-permitted-org?
  [token]
  (let [user-orgs (set (map :login (github-get "user/orgs" token)))
        permitted-orgs (set '(sharingio cncf kubernetes))]
    (empty? (clojure.set/intersection user-orgs permitted-orgs))))

(defroutes app-routes
  (GET "/" {session :session}
       (views/splash (:name (:username session))))

  (GET "/oauth"[code :as {session :session}]
       (if code
         (let [token (get-token code)
               {username :login
                fullname :name
                avatar :avatar_url
                :as user}(github-get "user" token)
               email (get-primary-email token)
               permitted-org-member (in-permitted-org? token)
               user {:username username
                     :fullname fullname
                     :avatar avatar
                     :email email
                     :permitted-org-member permitted-org-member}]
           (if (db/find-user username)
             (db/update-user user)
             (db/add-user user))
           (assoc (res/redirect (if (:project session) "/launch" "/"))
                  :session (merge session {:token token :user username})))))
  (route/not-found "Not Found"))

(def app
  (let [store (cookie/cookie-store {:key (env :session-secret)})]
    (wrap-defaults app-routes (assoc site-defaults :session {:store store}))))
