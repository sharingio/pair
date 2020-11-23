(ns client.spec
  (:require
   [clojure.spec.alpha :as s]
   [clojure.spec.test.alpha :as test]))

(def github-username-regex #"^([A-Za-z\d]+-)*[A-Za-z\d]+$")

(s/def ::username (s/and string? #(re-matches github-username-regex %)))
(s/valid? ::username "zachmandeville")
(s/valid? ::username "BobyMcBobs")
(s/valid? ::username "cool-h2and-57l-luke")
(s/valid? ::username "-should-be-false!")
