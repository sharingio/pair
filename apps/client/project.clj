(defproject client "0.1.0-SNAPSHOT"
  :description "FIXME: write description"
  :url "http://example.com/FIXME"
  :min-lein-version "2.0.0"
  :dependencies [[org.clojure/clojure "1.10.0"]
                 [org.clojure/test.check "1.1.0"]
                 [cheshire "5.10.0"]
                 [compojure "1.6.1"]
                 [hiccup "1.0.5"]
                 [environ "1.2.0"]
                 [clj-http "3.10.3"]
                 [ring/ring-defaults "0.3.2"]
                 [org.postgresql/postgresql "42.2.18.jre7"]
                 [seancorfield/next.jdbc "1.1.613"]]
  :plugins [[lein-ring "0.12.5"]
            [environ/environ.lein "0.2.1"]]
  :ring {:handler client.web/app
         :nrepl {:start? true}}
  :profiles
  {:dev {:dependencies [[javax.servlet/servlet-api "2.5"]
                        [ring/ring-mock "0.3.2"]]}})
