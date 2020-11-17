(ns client.github
  (:require [cheshire.core :as json]
            [environ.core :refer [env]]
            [clj-http.client :as http]))

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

(defn get-repo
  [project]
  (-> (http/get (str "https://api.github.com/repos/" project)
                {:headers {"accept" "application/json"}})
      :body (json/decode true)))
