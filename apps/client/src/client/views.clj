(ns client.views
  (:require [hiccup.page :refer [html5 include-css]]
            [client.db :as db]
            [environ.core :refer [env]]))

(def login-url (str "https://github.com/login/oauth/authorize?"
                    "client_id=" (env :oauth-client-id)
                    "&scope=read:user user:email read:org"))

(defn header
  [username]
  (if username
    (let [{:keys [full_name avatar_url]} (db/find-user username)]
    [:header
     [:a.home {:href "/"} "sharing.io"]
     [:nav
      [:p [:img {:src avatar_url}] [:a {:href "/logout"} "logout"]]]])
  [:header
   [:a.home {:href "/"} "sharing.io"]
   [:nav
    [:a {:href login-url} (if username username "login with github")]]]))

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
     [:p.tagline "Sharing is pairing!"]
     [:form {:action "/launch"
             :method :get
             :id "git-started"}
      [:label {:for "project"} "Enter a github repository"]
      [:input {:type "text"
               :name "project"
               :placeholder "user/repo"}]
      [:input {:type "submit"
               :value "Get Started!"}]]]]
   username))

(defn launch
  [username project]
  (layout
   [:main#launch
    [:h3 "launching"]] username))
