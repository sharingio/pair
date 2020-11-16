(ns client.views
  (:require [hiccup.page :refer [html5 include-css]]
            [environ.core :refer [env]]))

(def login-url (str "https://github.com/login/oauth/authorize?"
                    "client_id=" (env :oauth-client-id)
                    "&scope=read:user user:email read:org"))

(defn header
  [username]
  [:header
   [:a.home {:href "/"} "sharing.io"]
   [:nav
    [:a {:href login-url} (if username username "login with github")]]])

(defn layout
  [body username]
  (html5
   [:head
    [:meta {:charset 'utf-8'}]
    [:link {:rel "preconnect"
     :href "https://fonts.gstatic.com"}]
    [:link {:rel "stylesheet"
            :href "https://fonts.googleapis.com/css2?family=Manrope:wght@200;400;600;800&display=swap"}]
    [:meta {:name "viewport"
            :content "width=device-width"}]
    (include-css "/stylesheets/main.css")]
   [:body
    (header username)
    body]))

(defn splash
  [username]
  (layout
   [:main#splash
    [:section#cta
     [:p.tagline "Sharing is pairing!"]]]
   username))
