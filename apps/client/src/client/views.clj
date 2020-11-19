(ns client.views
  (:require [hiccup.page :refer [html5 include-css]]
            [hiccup.form :as form]
            [ring.util.anti-forgery :as util]
            [client.db :as db]
            [client.github :as gh]
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
  [body username &[refresh?]]
  (html5
   [:head
    [:meta {:charset 'utf-8'}]
    (when refresh? [:meta {:http-equiv "refresh" :content "20"}])
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
  (let [{:keys [html_url description]} (gh/get-repo project)
        {:keys [full_name email permitted_org_member]} (db/find-user username)]
  (layout
   [:main#launch
    [:h3 "Let's Collaborate on " project]
    [:p description]
    [:hr]
    (if permitted_org_member
         [:div
       [:h3 "Deploy to Packet"]
          (form/form-to
           [:post "/launch"]
           (util/anti-forgery-field)
        [:label {:for "type"} "Type"]
        (form/drop-down "type" '("Kubernetes")
                        "kubernetes")
        [:input {:type :hidden
                 :name "project"
                 :value project}]
        [:input {:type :hidden
                 :name "facility"
                 :value "sjc1"}]
        [:input {:type :hidden
                 :name "fullname"
                 :value full_name}]
        [:input {:type :hidden
                 :name "email"
                 :value email}]
        [:label {:for "guests"} "guests"]
        [:input {:type :text
                 :name "guests"
                 :id "guests"
                 :placeholder "users to invite (space separated)"}]
        [:label {:for "repos"} "Additional Repos"]
        [:input {:type :text
                 :name "repos"
                 :id "repos"
                 :placeholder "additional repos to add (space separated)"}]
        [:input {:type :submit :value "launch"}])]
         [:div "you aren't allowed"])]
     username)))

(defn project
  [username {:keys [project status]}]
  (let [{:keys [phase kubeconfig tmate]} (db/find-instance username project)]
  (layout
   [:main#project
    [:h3 "Pairing Box for " project]
    [:p phase]
    (when kubeconfig
      [:details
       [:summary "Your Kubeconfig"]
       [:pre kubeconfig]])
    (when tmate
      [:p tmate])
    ]
   username true)))
