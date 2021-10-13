(ns client.github
  (:require [cheshire.core :as json]
            [environ.core :refer [env]]
            [clojure.tools.logging :as log]
            [clojure.spec.alpha :as s]
            [clojure.spec.test.alpha :as test]
            [clj-http.client :as http]
            [client.spec :as spec])
(:use [slingshot.slingshot :only [throw+ try+]]))


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

(defn get
  [endpoint token]
  (-> (http/get (str "https://api.github.com/" endpoint)
                {:headers {"accept" "application/json"
                           "Authorization" (str "token " token)}})
      :body (json/decode true)))

(defn get-raw-info
  [token]
  {:user (get "user" token)
   :emails (get "user/emails" token)
   :token token
   :orgs (get "user/orgs" token)})

(s/fdef primary-email
  :args (s/cat :emails :gh/emails)
  :ret (s/nilable :client.spec/email))
(defn primary-email
  [emails]
  (:email (first (filter #(= (:primary %) true) emails))))

(s/fdef in-permitted-org?
  :args (s/cat :orgs :gh/orgs)
  :ret boolean?)
(defn in-permitted-org?
  [orgs]
  (let [permitted-orgs (set (clojure.string/split (env :pair-permitted-orgs) #" "))
        user-orgs (set (map :login orgs))]
    ((complement empty?) (clojure.set/intersection user-orgs permitted-orgs))))

(s/fdef is-admin?
  :args (s/cat :emails :gh/emails)
  :ret (s/nilable boolean?))
(defn is-admin?
  "do emails include ii.coop, indicating admin"
  [emails]
  (let [addresses (->> emails (filter :verified) (map :email))]
  (some #(clojure.string/ends-with? % (str "@" (env :pair-admin-email-domain))) addresses)))

(s/fdef user-info
  :args (s/cat :raw-info :gh/raw-info)
  :ret :client.spec/user)
(defn user-info
  [{{:keys [login name avatar_url html_url]} :user
    emails :emails
    orgs :orgs
    token :token}]
  (println "orgs: " orgs)
  {:username login
   :fullname name
   :email (primary-email emails)
   :emails emails
   :token token
   :profile html_url
   :avatar avatar_url
   :permitted-member (in-permitted-org? orgs)
   :admin-member (is-admin? emails)})

(s/fdef get-user-info
  :args (s/cat :code string?)
  :ret :client.spec/user)
(defn get-user-info
  [code]
  (-> code get-token get-raw-info user-info))


(comment
  (def user (get-user-info "376cab7a8724c40cebe8"))
user

)
