(ns syme.html
  (:require [clojure.java.io :as io]
            [tentacles.repos :as repos]
            [tentacles.users :as users]
            [environ.core :refer [env]]
            [syme.instance :as instance]
            [syme.packet :as packet]
            [hiccup.page :refer [html5 include-css]]
            [hiccup.form :as form]))

(def login-url (str "https://github.com/login/oauth/authorize?"
                    "client_id=" (env :oauth-client-id)))

(defn layout [body username & [project status]]
  (html5
   [:head
    [:meta {:charset "utf-8"}]
    (when (some? status)
      [:meta {:http-equiv "refresh" :content "20"}])
    [:title (if project (str project " - Syme") "Syme")]
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
    [:p {:style "margin-bottom: 700px;"} "&nbsp;"]] username))

(defn faq [username]
  (layout (slurp (io/resource "faq.html")) username))

(defn launch [username repo-name identity credential]
  (let [repo (try (apply repos/specific-repo (.split repo-name "/"))
                  (catch Exception _))]
    (when-not (:name repo)
      (throw (ex-info "Repository not found." {:status 404})))
    (layout
      [:div
       [:h3.project [:a {:href {:html_url repo}} repo-name]]
       [:p#desc (:description repo)]
       [:hr]
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
        [:label {:for "guests"} "guests"]
        [:input {:type :text
                 :name "guests"
                 :id "guests"
                 :placeholder "users to invite (space separated)"}]
        [:input {:type :submit :value "launch"}]]]
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
          (when (and (> level 1) (packet/kubeconfig-available? instance_id))
          [:li [:strong "kubeconfig is available "]
           [:a {:href (str "/project/"project"/kubeconfig") :download true} (str "download " instance_id"-kubeconfig")]])
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
      (map #(render-instance-info % link-syme-project) instances)
      [:p "You have no instances"])
    [:hr]
    [:p "You may want to periodically check your "
     [:a {:href
          "https://console.aws.amazon.com/ec2/home?region=us-west-2#s=Instances"}
      "AWS EC2 console"]
     " to ensure you aren't billed for instances you intended to stop that"
     " stayed running due to problems with Syme. In particular this happens"
     " due to timeout errors."
     ]]
   username "Status"))

  ;; ;; TODO: remove inline styles
  ;; [:p {:id "haltbutton" :style "float: right; margin: -7px 0;"}
  ;;  [:button {:onclick "show_halt()"} "Halt"]]
  ;; [:div {:id "halt" :style "float: right; clear: right; display: none"}
  ;;  [:button {:onclick "hide_halt();"} "Cancel"]
  ;;  [:button {:onclick (format "halt('%s')" project)} "Confirm"]]
  ;; [:p {:id "ip" :class status
  ;;      :title "Send this command to the users you've invited."}
  ;;  [:tt "ssh ii@" (or dns ip)]]]
;; TODO bring back the guest invites
;; [:hr]
;; [:ul {:id "users"}
;;  (for [u invitees]
;;    [:li [:a {:href (str "https://github.com/" u)}
;;          [:img {:src (icon u) :alt u :title u :height 80 :width 80}]]])]]
