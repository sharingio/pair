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
    [:header#top
     [:h1 [:a.home {:href "/"} "sharing.io"]]
     [:nav
      [:p [:img {:src avatar_url}] [:a {:href "/logout"} "logout"]]]])
  [:header
   [:h1 [:a.home {:href "/"} "sharing.io"]]
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
     [:div
      [:a {:href "/new"} "New"]
      [:a {:href "/instances"} "All"]]]]
   username))

(defn new-box-form
  [{:keys [fullname email username]}]
  (form/form-to {:id "new-box"}
   [:post "/new"]
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
            :placeholder "additional repos to add (space separated)"}]]
   [:div.form-group
   [:label {:for "guests"} "guests"]
   [:input {:type :text
            :name "guests"
            :id "guests"
            :placeholder "users to invite (space separated)"}]]
   [:input {:type :submit :value "launch"}]))

(defn new
  [user]
  (layout
   [:main
    [:header
     [:h2 "Create a new Pairing Box"]]
    (new-box-form user)]
   (:username user)))

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
  (when (and tmate-ssh tmate-web)
    [:section#tmate
     [:h3 "Pairing Session Ready"]
     [:a.tmate.action {:href tmate-web} "Join Pair"]
     [:aside
      [:p "Or join via ssh:"
      [:pre tmate-ssh]]]]))

(defn status
  [{:keys [facility type phase]}]
  [:section#status
   [:h3 "Status: " phase]
    [:p  type " instance"]
    [:p "deployed at " facility]])


(defn instance
  [instance username]
  (layout
   [:main#instance
   [:header
    [:h2 "Status for "(:instance-id instance)]]
    [:article
    (tmate instance)
     (status instance)
    (kubeconfig-box (:kubeconfig instance))
    (when (= (:owner instance) username)
      [:a.action.delete {:href (str "/instances/id/"(:instance-id instance)"/delete")}
       "Delete Instance"])]]
   username))

(defn delete-instance
  [{:keys [instance-id]} username]
  (layout
   [:main#delete-instance
    [:header
     [:h2 "Delete "instance-id]]
    [:article
     [:h3 "Do you really want to delete this box?"]
     (form/form-to {:id "delete-box"}
                   [:post (str "/instances/id/"instance-id"/delete")]
                   (util/anti-forgery-field)
                   [:input {:type :hidden
                            :name "instance-id"
                            :value instance-id}]
                   [:input {:type :submit
                            :name "confirm"
                            :value (str "Delete " instance-id)}])]]
   username))


(defn all-instances
  [instances {:keys [username]}]
  (let [[owner guest] ((juxt filter remove) #(= (:owner %) username) instances)]
    (layout
     [:main
      [:header
       [:h2 "Your Instances"]]
      [:section#owner
       [:h3 "Created by You"]
       [:ul
        (for [instance owner]
          [:li [:a {:href (str "/instances/id/"(:instance-id instance))}
           (:instance-id instance)]])]]
      (when guest
      [:section#guest
       [:h3 "Shared with You"]
       (for [instance guest]
         [:a {:href (str "/instances/id/"(:instance-id instance))}
          (:instance-id instance)])])]
     username)))
