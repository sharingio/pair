(ns client.spec
  (:require
   [clojure.spec.alpha :as s]
   [clojure.spec.test.alpha :as test]))

(def github-username-regex #"^([A-Za-z\d]+-)*[A-Za-z\d]+$")
(def email-regex #"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,63}$")

(s/def ::email (s/and string? #(re-matches email-regex %)))
(s/def ::username (s/and string? #(re-matches github-username-regex %)))
(s/def ::permitted-member boolean?)
(s/def ::avatar string?)
(s/def ::profile-url string?)
(s/def ::gh-token string?)

(s/def ::user (s/keys :req-un [::username ::email ::permitted-member ::avatar ::profile-url ::gh-token]))
(s/valid? ::user {:username "zachmandeville"
                  :email "zz@ii.coop"
                  :permitted-member true
                  :avatar "https://github.com/avatar"
                  :profile-url "https://github.com/zachmandeville"
                  :gh-token "zjdkj198203ekj39802"})
