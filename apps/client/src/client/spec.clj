(ns client.spec
  (:require
   [clojure.spec.alpha :as s]
   [clojure.spec.test.alpha :as test]
   [java-time :as time]
   [com.gfredericks.test.chuck :as chuck]
   [com.gfredericks.test.chuck.generators :as gen']))

(defn fn-string-from-regex
  "Return a function that produces a generator for the given
  regular expression string."
  [regex]
  (fn []
    (gen'/string-from-regex regex)))



(def github-username-regex #"^([A-Za-z\d]+-)*[A-Za-z\d]+$")
;; this is an email-regex from the clojure.spec guide, but uses anchors which are
;; unsupported in test.chuck
;; (def email-regex #"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,63}$")

;; this is thanks to clojure slack and seancorfield
(def email-regex
  "Sophisticated regex for validating an email address."
  (re-pattern
   (str "(([^<>()\\[\\]\\\\.,;:\\s@\"]+(\\.[^<>()\\[\\]\\\\.,;:\\s@\"]+)*)|"
        "(\".+\"))@((\\[[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}\\])|"
        "(([a-zA-Z\\-0-9]+\\.)+[a-zA-Z]{2,}))")))

;; we generate our own type of emails to ensure our test functions will allways have a valid (if bizarre) email
(s/def ::email (s/with-gen (s/and string?
                                  #(re-matches email-regex %))
                 (fn-string-from-regex email-regex)))

(s/def ::username (s/and string? #(re-matches github-username-regex %)))

(s/def :gh/token string?)
(s/def :gh/html_url string?)
(s/def :gh/url string?)
(s/def :gh/name string?)
(s/def :gh/location string?)
(s/def :gh/login ::username)
(s/def :gh/user (s/keys :req-un [::email ::name ::login]))

(s/def :gh/primary (s/nilable boolean?))
(s/def :gh/verified (s/nilable boolean?))
(s/def :gh/visibility (s/nilable string?))
(s/def :gh/email (s/keys :req-un [::email :gh/primary :gh/verified :gh/visibility]))
(s/def :gh/emails (s/coll-of :gh/email :distinct true))
(s/def :gh/org (s/map-of keyword? any?))
(s/def :gh/orgs (s/coll-of :gh/org))
(s/def :gh/raw-info (s/keys :req-un [:gh/user :gh/emails :gh/orgs]))

(s/def ::permitted-member boolean?)
(s/def ::admin-member boolean?)
(s/def ::avatar string?)
(s/def ::fullname string?)
(s/def ::profile string?)

(s/def ::user (s/keys :req-un [::username ::fullname ::email ::permitted-member ::avatar ::profile ::admin-member]))

;; TODO: proper spec for 2020-11-30T21:32:44Z
(s/def :cluster/timestamp string?)
