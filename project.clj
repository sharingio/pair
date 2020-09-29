(defproject syme "1.1.0"
  :description "Instant collaboration on GitHub projects over tmux."
  :url "http://syme.herokuapp.com"
  :license "Eclipse Public License 1.0"
  :dependencies [[org.clojure/clojure "1.10.0"]
                 [com.amazonaws/aws-java-sdk "1.3.33"
                  :exclusions [org.apache.httpcomponents/httpclient
                               commons-codec]]
                 [compojure "1.6.2"]
                 [ring/ring-core "1.8.1"]
                 [ring/ring-jetty-adapter "1.8.1"]
                 [hiccup "1.0.5"]
                 [tentacles "0.5.1"]
                 [clj-http "3.10.3" :exclusions [commons-logging]]
                 [cheshire "5.10.0"]
                 [environ "1.2.0"]
                 [lib-noir "0.9.9"]
                 [org.postgresql/postgresql "42.2.16.jre7"]
                 [org.clojure/java.jdbc "0.2.3"]]
  :uberjar-name "syme-standalone.jar"
  :target-path "target/%s/"
  :min-lein-version "2.0.0"
  :plugins [[environ/environ.lein "0.2.1"]]
  :hooks [environ.leiningen.hooks]
  :profiles {:production {:env {:production true}}})
