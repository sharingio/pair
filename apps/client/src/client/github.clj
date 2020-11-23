(ns client.github
  (:require [cheshire.core :as json]
            [environ.core :refer [env]]
            [clojure.tools.logging :as log]
            [clojure.spec.alpha :as s]
            [clojure.spec.test.alpha :as test]
            [clj-http.client :as http])
(:use [slingshot.slingshot :only [throw+ try+]]))


(def github-username-regex #"^([A-Za-z\d]+-)*[A-Za-z\d]+$")

(s/def ::username (s/and string? #(re-matches github-username-regex %)))

(s/def ::gh-oauth-code string?)

(s/fdef get-token
  :args (s/cat :code ::gh-oauth-code)
  :ret (s/nilable string?))


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
  (try+ (-> (http/get (str "https://api.github.com/repos/" project)
                {:headers {"accept" "application/json"}})
            :body (json/decode true))
        (catch [:status 404] {:keys [request-time headers body]}
          (log/warn "404" request-time project headers body))))
