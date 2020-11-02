(ns sharingio.html
  (:require [clojure.java.io :as io]
            [tentacles.repos :as repos]
            [tentacles.users :as users]
            [environ.core :refer [env]]
            [sharingio.packet :as packet]
            [hiccup.page :refer [html5 include-css]]
            [hiccup.form :as form]))

(def login-url (str "https://github.com/login/oauth/authorize?"
                    "client_id=" (env :oauth-client-id)
                    "&scope=read:user user:email read:org"))

(defn layout [body username & [project status]]
  (html5
   [:head
    [:meta {:charset "utf-8"}]
    (when (some? status)
      [:meta {:http-equiv "refresh" :content "20"}])
    [:title (if project (str project " - Sharingio") "Sharingio")]
    (include-css "/stylesheets/style.css" "/stylesheets/base.css"
                 "/stylesheets/skeleton.css")
    (include-css "https://fonts.googleapis.com/css?family=Passion+One:700")]
   [:body
    (if-let [account (:analytics-account env)]
      [:script {:type "text/javascript"} (-> (io/resource "analytics.js")
                                             (slurp) (format account))])
    [:div#header
     [:h1.container [:a {:href "/"} "Sharing is Pairing"]]]
    [:div#content.container body
     [:div#footer
      [:p [:a {:href "/faq"} "About"]
       " | " [:a {:href "https://github.com/sharingio/pair"}
              "Source"]
       " | " (if username
               (list [:a {:href "/all"} "All Instances"] " | "
                     [:a {:href "/logout"} "Log out"])
               [:a {:href login-url} "Log in"])]]]]))

(defn splash [username]
  (layout
   [:div
    [:img {:src "/splash.png"
           :style "position: absolute; z-index: -1; top: -10px; left: -30px;"}]
    [:form {:action "/launch" :method :get :id "splash"
            :style "position: absolute; top: 257px; left: -20px; width: 440px;"}
     [:input {:type :submit :value "Collaborate on a GitHub project"
              :style "width: 48%; float: right;"}]
     [:input {:type :text :name "project"
              :style "width: 48%; height: 14px; font-weight: bold;"
              :placeholder "user/project"}]]
    [:p {:style "margin-bottom: 700px;"} "&nbsp;"]
    [:p "Hello!"]
    ] username))

(defn faq [username]
  (layout (slurp (io/resource "faq.html")) username))

(defn launch [username repo-name identity credential details]
  (let [repo (try (apply repos/specific-repo (.split repo-name "/"))
                  (catch Exception _))]
    (when-not (:name repo)
      (throw (ex-info "Repository not found." {:status 404})))
    (layout
      [:div
       [:h3.project [:a {:href {:html_url repo}} repo-name]]
       [:p (str "Hello, " (:fullname details))]
       [:p#desc (:description repo)]
       [:hr]
       (if (:sharingio_member details)
         [:div
       [:h3 "Deploy to Packet"]
       [:form {:action "/launch" :method :post}
        [:label {:for "type"} "Type"]
        (form/drop-down "type" '("Kubernetes")
                        "kubernetes")
        [:input {:type :hidden
                 :name "project"
                 :value repo-name}]
        [:input {:type :hidden
                 :name "facility"
                 :value "sjc1"}]
        [:input {:type :hidden
                 :name "fullname"
                 :value (:fullname details)}]
        [:input {:type :hidden
                 :name "email"
                 :value (:email details)}]
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
        [:input {:type :submit :value "launch"}]]]
         [:div "you aren't allowed"])]
     username repo-name)))

(defonce icon (memoize (comp :avatar_url users/user)))

(defn- link-syme-project [project]
  (format "/project/%s" project))

(defn- link-github-project [project]
  (format "https://github.com/%s" project))

(defn- render-instance-info [{:keys [project description]} phase link-project]
  [:div
   [:p {:id "status"} phase]
   [:h3.project [:a {:href (link-project project)} project]]
   [:p {:id "desc"} description]])

(defn instance [username {:keys [project description invitees instance_id type facility]
                          :as instance-info} {:keys [level phase cluster humacs] :as status}]
  (layout
   [:div
    (render-instance-info instance-info phase link-github-project)
    [:hr]
      [:div
       [:p (str "A "type" instance on Equinix, using facility "facility)]
        [:h4 "status"]
       [:em "This page refreshes every 20 seconds to get current status of your instance"]
       (if (nil? cluster)
         [:ul [:li [:strong "The cluster is not yet ready"]]]
         [:ul
          [:li [:strong (str "The Cluster is " cluster)]]
          [:li [:a {:href (str "/project/"project"/delete")}"delete this instance"]]
          (when (and (> level 1) (packet/kubeconfig-available? instance_id))
          [:li [:strong "kubeconfig is available "]
           [:a {:href (str "/project/"project"/kubeconfig") :download (str instance_id "-kubeconfig")} (str "download " instance_id"-kubeconfig")]])
          (when (and (>= level 2)(< level 5))
            [:li [:strong "Configuring cluster for pairing"]])
          (when (= level 5)
            [:li [:strong "Pairing setup ready"]
             [:div [:strong "paste this into your terminal to join tmate session: "]
              [:pre (packet/get-tmate instance_id )]]])])]]
username project status))


(defn all [username instances]
  (layout
   [:div [:h3 "All Instances"]
    (if instances
      [:ul
      (for [{:keys [project status]} instances]
        [:li [:a {:href (str "project/" project)}
              [:strong (str project": ")]] status])])]
   username "Status"))

;; TODO bring back the guest invites
;; [:hr]
;; [:ul {:id "users"}
;;  (for [u invitees]
;;    [:li [:a {:href (str "https://github.com/" u)}
;;          [:img {:src (icon u) :alt u :title u :height 80 :width 80}]]])]]
