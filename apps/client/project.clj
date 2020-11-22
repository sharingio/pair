(defproject client "0.1.0-SNAPSHOT"
  :description "Sharingio client: Web frontend for sharingio pair box creation"
  :url "https://sharing.io"
  :min-lein-version "2.0.0"
  :dependencies [[org.clojure/clojure "1.10.0"]
                 [org.clojure/test.check "1.1.0"]
                 [cheshire "5.10.0"]
                 [compojure "1.6.1"]
                 [io.forward/yaml "1.0.10"]
                 [hiccup "1.0.5"]
                 [environ "1.2.0"]
                 [clj-http "3.10.3"]
                 [org.clojure/tools.logging "1.1.0"]
                 [ring/ring-defaults "0.3.2"]]
  :plugins [[lein-ring "0.12.5"]
            [environ/environ.lein "0.2.1"]]
  :ring {:handler client.web/app
         :port 5000
         :nrepl {:start? true}
         :autoload? true}
  :profiles
  {:dev {:dependencies [[javax.servlet/servlet-api "2.5"]
                        [ring/ring-mock "0.3.2"]]}})
