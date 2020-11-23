(ns client.spec
  (:require
   [clojure.spec.alpha :as s]
   [clojure.spec.test.alpha :as test]))

(def github-username-regex #"^([A-Za-z\d]+-)*[A-Za-z\d]+$")
(def email-regex #"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,63}$")

(s/def ::email (s/and string? #(re-matches email-regex %)))
(s/def ::username (s/and string? #(re-matches github-username-regex %)))

(s/def :gh/token string?)
(s/def :gh/primary (s/nilable boolean?))
(s/def :gh/verified (s/nilable boolean?))
(s/def :gh/email (s/keys :req-un [::email :gh/primary :gh/verified ::visibility]))
(s/def :gh/emails (s/coll-of :gh/email))
(s/def :gh/user (s/keys :req-un [::html_url ::email ::name ::login ::url ::location]))
(s/def :gh/org (s/keys :req-un [::description ::login
                                ::node_id ::id
                                ::url ::events_url
                                ::issues_url ::hooks_url
                                ::members_url ::public_members_url
                                ::repos_url ::avatar_url]))
(s/def :gh/orgs (s/coll-of :gh/org))
(s/def :gh/raw-info (s/keys :req-un [:gh/orgs :gh/user :gh/emails]))

(s/def ::permitted-member boolean?)
(s/def ::avatar string?)
(s/def ::profile-url string?)
(s/def ::user (s/keys :req-un [::username ::email ::permitted-member ::avatar ::profile-url :gh/token]))
