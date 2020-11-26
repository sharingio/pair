(ns client.views
  (:require [hiccup.page :refer [html5 include-css]]
            [hiccup.form :as form]
            [ring.util.anti-forgery :as util]
            [client.github :as gh]
            [environ.core :refer [env]]))

(def login-url (str "https://github.com/login/oauth/authorize?"
                    "client_id=" (env :oauth-client-id)
                    "&scope=read:user user:email read:org"))

(defn header
  [{:keys [avatar username] :as user}]
  (if user
    [:header#top
     [:h1 [:a.home {:href "/"} "sharing.io"]]
     [:nav
      [:a.btn.alpha {:href "/instances/new"} "New"]
      [:a.btn.beta {:href "/instances"} "All"]
      [:p [:img {:width "50px" :src avatar :alt (str "avatar icon for "username)}]
       [:a.logout {:href "/logout"} "logout"]]]]
  [:header#top
   [:h1 [:a.home {:href "/"} "sharing.io"]]
   [:nav
    [:a {:href login-url} "login with github"]]]))

(defn layout
  [body user &[refresh?]]
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
    (header user)
    body]))

(defn splash
  [{:keys [username] :as user}]
  (layout
   [:main#splash
    [:section#cta
     [:p.tagline "Sharing is pairing!"]
     (when username
     [:div
      [:a {:href "/instances/new"} "New"]
      [:a {:href "/instances"} "All"]])]]
   user))

(defn new-box-form
  [{:keys [fullname email username]}]
  (form/form-to {:id "new-box"}
   [:post "/instances/new"]
   (util/anti-forgery-field)
   [:label {:for "type"} "Type"]
   (form/drop-down "type" '("Kubernetes")
                   "kubernetes")
   [:input {:type :hidden
            :name "facility"
            :value "sjc1"}]
   [:input {:type :hidden
            :name "fullname"
            :value fullname}]
   [:input {:type :hidden
            :name "email"
            :value email}]
   [:div.form-group
   [:label {:for "repos"} "Repos to include"]
   [:input {:type :text
            :name "repos"
            :id "repos"
            :placeholder "additional repos to add (space separated)"
            :pattern "^[^,]*[^ ,][^,]*$"
            :title "Repos separated by white space"
            }]
    [:p.helper "separate each repo with whitespace"]]
   [:div.form-group
   [:label {:for "guests"} "guests"]
   [:input {:type :text
            :name "guests"
            :id "guests"
            :placeholder "github users to invite (space separated)"
            :pattern "^[^,]*[^ ,][^,]*$"
            :title "github usernames separated by white space"
            }]
    [:p.helper "please add github usernames, separated by whitespace"]]
   [:div.form-group
    [:label {:form "envvars"} "Environment Variables"]
    [:textarea {:name "envvars"
                :id "envvars"
                :placeholder "PAIR=sharing\nSHARE=pairing"}]
    [:p.helper "Add env vars as KEY=value, with each new variable on its own line."]]
   [:input {:type :submit :value "launch"}]))

(defn new
  [user]
  (layout
   [:main
    [:header
     [:h2 "Create a new Pairing Box"]]
    (new-box-form user)]
   user))

(defn kubeconfig-box
  [kubeconfig]
  (if kubeconfig
    [:section#kubeconfig
    [:h3 "Kubeconfig available"]
  [:details
   [:summary "See Full Kubeconfig"]
   [:pre kubeconfig]]]
    [:section#kubeconfig
     [:h3 "Kubeconfig not yet available"]]))

(defn tmate
  [{:keys [tmate-ssh tmate-web]}]
  (if(= "Not ready to fetch tmate session" tmate-web)
    [:section#tmate
     [:h3 "Pairing Session not yet Ready"]]
    [:section#tmate
     [:h3 "Pairing Session Ready"]
     [:a.tmate.action {:href tmate-web
                       :target "_blank"
                       :rel "noreferrer noopener"} "Join Pair"]
     [:aside
      [:p "Or join via ssh:"
      [:pre tmate-ssh]]]]))

(defn status
  [{:keys [facility type phase sites]}]
  [:section#status
   [:h3 "Status: " phase]
    [:p  type " instance"]
   [:p "deployed at " facility]
   [:p "Sites Available"]
   [:ul
    (for [site sites]
      [:li [:a {:href site
                :target "_blank"
                :rel "noreferrer noopener"} site]])]])


(defn instance
  [instance {:keys [username admin-member] :as user}]
  (layout
   [:main#instance
   [:header
    [:h2 "Status for "(:instance-id instance)]]
    [:article
    (tmate instance)
     (status instance)
    (kubeconfig-box (:kubeconfig instance))
    (when (or (= (:owner instance) username) admin-member)
      [:a.action.delete {:href (str "/instances/id/"(:instance-id instance)"/delete")}
       "Delete Instance"])]]
   user))

(defn delete-instance
  [{:keys [instance-id]} user]
  (layout
   [:main#delete-instance
    [:header
     [:h2 "Delete "instance-id"?"]]
    [:article
     [:h3 "Do you really want to delete this box?"]
     (form/form-to {:id "delete-box"}
                   [:post (str "/instances/id/"instance-id"/delete")]
                   (util/anti-forgery-field)
                   [:input {:type :hidden
                            :name "instance-id"
                            :value instance-id}]
                   [:input.action.delete {:type :submit
                            :name "confirm"
                            :value (str "Delete " instance-id)}])]]
   user))


(defn all-instances
  [instances {:keys [username admin-member] :as user}]
  (let [[owner rest] ((juxt filter remove) #(= (:owner %) username) instances)
        [guest other] ((juxt filter remove) #(some #{username} (:guests %)) rest)]
    (layout
     [:main#all-instances
      [:header
       [:h2 "Your Instances"]]
      [:article
      [:section#owner
       [:h3 "Created by You"]
       [:ul
        (for [instance owner]
          [:li [:a {:href (str "/instances/id/"(:instance-id instance))}
           (:instance-id instance)] [:em (:phase instance)]])]]
      (when guest
      [:section#guest
       [:h3 "Shared with You"]
       [:ul
       (for [instance guest]
         [:li [:a {:href (str "/instances/id/"(:instance-id instance))}
               (:instance-id instance)] [:em (:phase instance)]])]])
       (when (and admin-member other)
         [:section#admin
          [:h3 "All Other Instances"]
          [:ul
           (for [instance other]
             [:li [:a {:href (str "/instances/id/"(:instance-id instance))}
                   (:instance-id instance)] [:em (:phase instance)]])]])]]
     user)))
