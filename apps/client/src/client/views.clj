(ns client.views
  (:require [hiccup.page :refer [html5 include-css]]
            [hiccup.form :as form]
            [ring.util.anti-forgery :as util]
            [client.github :as gh]
            [clojure.string :as str]
            [environ.core :refer [env]]))

(defn loginURL
  [rw?]
  (str "https://github.com/login/oauth/authorize?"
                    "client_id=" (env :oauth-client-id)
                    "&scope=read:user user:email read:org" (when rw? " repo")))

(defn code-box
  [id val]
  [:div.code-box
   [:pre {:id id} val]
   [:button {:id (str "copy-"id)} "&#128203;"]])

(defn header
  [{:keys [avatar username permitted-member] :as user}]
  (if (and user (not (= username "guest")))
    [:header#top
     [:h1 [:a.home {:href "/"} "sharing.io"]]
     [:nav
      (when permitted-member
        (list
        [:a.btn.beta {:href "/instances/new"} "New"]
        [:a.btn.alpha {:href "/instances"} "All"]))
      [:p [:img {:width "50px" :src avatar :alt (str "avatar icon for "username)}]
       [:a.logout {:href "/logout"} "logout"]]]]
  [:header#top
   [:h1 [:a.home {:href "/"} "sharing.io"]
    (when (= "guest" username)[:sup "public link"])]
   [:nav
    (when (nil? user) [:a {:href "/login"} "login with github"])]]))

(defn layout
  [body user &[refresh?]]
  (html5
   [:head
    [:meta {:charset 'utf-8'}]
    (when refresh? [:noscript [:meta {:http-equiv "refresh" :content "15"}]])
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

(defn login
  [user]
  (layout
   [:main
    [:header
     [:h2 "Login"]]
    [:nav#login-options
     [:ul
      [:li
       [:a.button.action.strong.long {:href (loginURL true)} "login"]
       [:em.helper "Elevated permissions, with full read/write on any of your repos.  This permission is passed to the cluster, letting you easily push and pull from within it."]]
      [:li
       [:a.button.action.long {:href (loginURL false)}"login (read-only)"]
       [:em.helper "Login with minimal github permissions. We request access to your org and emails, to see if you are a permitted member and an admin member."]]
      [:li
       [:a.button.action.alert {:href "/logout"} "logout"]
       [:em.helper "Logout from sharing.io"]]]]]
   user))


(defn splash
  [{:keys [permitted-member] :as user}]
  (layout
   [:main#splash
    [:section#cta
     [:p.tagline "Sharing is Pairing"]
     (if permitted-member
     [:div
      [:a {:href "/instances/new"} "New"]
      [:a {:href "/instances"} "All"]]
     [:div
      [:p "To use sharing.io, you must be a member of a permitted github org."]]
     )]]
   user))

(defn new-box-form
  [{:keys [fullname email username admin-member]}]
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
   [:input {:type :hidden
            :id "timezone"
            :name "timezone"
            :value "Pacific/Auckland"}]
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
   (when admin-member
     [:div.form-group
      [:label {:form "name"} "Custom Name for Instance"]
      [:input {:name "name"
                  :id "name"
                  :placeholder "coolbox-123"}]
      [:p.helper "You can set a custom name for your box, which will be used in dns."]])
   [:input {:type :submit :value "launch"}]))

(defn new
  [user]
  (layout
   [:main
    [:header
     [:h2 "Create a new Pairing Box"]]
    (new-box-form user)
    ;; This will set the timezone field to the timezone of the client browser.  If js disabled, timezone is Pacific/Auckland
    [:script "document.querySelector('input#timezone').value=(new Intl.DateTimeFormat).resolvedOptions().timeZone;"]]
   user))

(defn kubeconfig-box
  [{:keys [kubeconfig uid instance-id]}]
  (if kubeconfig
    [:section#kubeconfig
     [:h3 "Kubeconfig available "]
     [:a#kc-dl {:href (str "https://"(env :domain)"/public-instances/"uid"/"instance-id"/kubeconfig")
                                       :download (str instance-id"-kubeconfig")} "download"]
     [:p "you can attach to the cluster immediately with this command: "]
      (code-box "kc-command"
                (str
                 "export KUBECONFIG=$(mktemp -t kubeconfig) ; curl -s "
                 "https://"(env :domain)"/public-instances/"uid"/"instance-id"/kubeconfig > \"$KUBECONFIG\""
                 " ; kubectl api-resources"))
  [:details
   [:summary "See Full Kubeconfig"]
   (code-box "kc" kubeconfig)]]
    [:section#kubeconfig
     [:h3 "Kubeconfig not yet available"]]))

(defn envvars
  [{:keys [envvars]}]
  (if envvars
    [:details
     [:summary "See Environment variables set on launch"]
     (code-box "env" envvars)]
    )
  )

(defn tmate
  [{:keys [tmate-ssh tmate-web]}]
  (if(or (= "Not ready to fetch tmate session" tmate-web) (empty? tmate-ssh))
    [:section#tmate
     [:h3 "Pairing Session not yet Ready"]]
    [:section#tmate
     [:h3 "Pairing Session Ready"]
     [:a.tmate.action {:href tmate-web
                       :target "_blank"
                       :rel "noreferrer noopener"} "Join Pair"]
     [:aside
      [:p "Join via ssh:"]
      (code-box "tmate-ssh" tmate-ssh)]]))

(defn status
  [{:keys [facility type phase sites dns cert]}]
  [:section#status
   [:h3#phase "Status: " phase]
   [:p#type  type " instance"]
   [:p#facility "deployed at " facility]
   [:h3 "Sites Available"]
   [:ul#sites-available
    (for [site sites]
      [:li [:a {:href site
                :target "_blank"
                :rel "noreferrer noopener"} site]])]])

(defn instance-header
  [{:keys [guests uid instance-id repos age owner]} {:keys [username]}]
   [:header
    [:h2 "Status for "instance-id
     (when (not (= "guest" username))
       (if (nil? uid)
         [:a#public-link.btn.action.hidden {:href (str "/public-instances/"uid"/"instance-id)
                                            :target "_blank"
                                            :rel "noreferrer nofollower"} "Get Public Link"]
         [:a#public-link.btn.action {:href (str "/public-instances/"uid"/"instance-id)
                                            :target "_blank"
                                            :rel "noreferrer nofollower"} "Get Public Link"]))]
    [:div.info
     [:em "This instance was brought up by " owner]
     [:em#age age]
     (when (> (count (filter (complement empty?) guests)) 0)
       [:div.detail
        [:h3 "Shared with:"]
        [:ul.guests
         (for [guest guests]
           [:li [:a {:href (str "https://github.com/"guest)}guest]]
           )]])
     (when (> (count (filter (complement empty?) repos)) 0)
       [:div.detail
        [:h3 "Loaded with:"]
        [:ul.repos
         (for [repo repos]
           [:li [:a {:href (if (re-find #"^(http)(.)+//" repo) repo (str "https://github.com/" repo))}
                 repo]])]])]])

(defn instance-admin
  [{:keys [owner facility uid instance-id]} {:keys [admin-member username]}]
  (when (or (= owner username) admin-member)
    [:section.admin-actions
     [:a.action.delete
      {:href (str "/instances/id/"instance-id"/delete")}
      "Delete Instance"]
     [:h3 "SOS ssh:"]
     (code-box "sos-ssh" (str "ssh "uid"@sos."facility".platformequinix.com"))]))

(defn instance
  [instance user]
  (layout
   (list
   [:main#instance
    (instance-header instance user)
    [:article
     [:section.status
    (tmate instance)
    (kubeconfig-box instance)
    (envvars instance)]
    [:aside
     (status instance)
     (instance-admin instance user)]]]
    [:script {:src "/status.js"}]
    [:script {:src "/clipboard.js" :type "module"}])
   user true))

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

(defn instance-li
  "used in all instances page, short list of info about instance!"
  [{:keys [instance-id phase age]}]
  [:li.instance [:a {:href (str "/instances/id/"instance-id)}
        instance-id] [:em.phase phase]
   [:p.age age]])


(defn all-instances
  [instances {:keys [username admin-member] :as user}]
  (let [[owner rest] ((juxt filter remove) #(= (:owner %) username) instances)
        [guest other] ((juxt filter remove) #(some #{username} (:guests %)) rest)]
    (layout
     [:main#all-instances
      [:header
       [:h2 "Your Instances"]]
      [:article
       (when owner
      [:section#owner
       [:h3 "Created by You"]
       [:ul
        (for [instance owner]
          (instance-li instance))]])
      (when guest
      [:section#guest
       [:h3 "Shared with You"]
       [:ul
       (for [instance guest]
         (instance-li instance))]])
       (when (and admin-member other)
         [:section#admin
          [:h3 "All Other Instances"]
          [:ul
           (for [instance other]
             (instance-li instance))]])]]
     user)))
