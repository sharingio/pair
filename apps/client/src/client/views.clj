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
      (when (and permitted-member (seq (:ssh-keys user)))
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
       ;; if has no ssh-keys
       (if (seq (:ssh-keys user))
         [:div#action-buttons
          [:a {:href "/instances/new"} "New"]
          [:a {:href "/instances"} "All"]]
         [:div.missing-keys
          [:p "Pair requires your GitHub account have SSH keys added to it."
           [:a {:target "blank" :rel "noreferrer" :href "https://docs.github.com/en/authentication/connecting-to-github-with-ssh/adding-a-new-ssh-key-to-your-github-account"} "Add keys to your account"]]]
         )
       [:div.display-block
        [:div#more-info.display-block
         [:p
          "Sharable Pairing Environments (on Equinix Metal)."]
         [:p
          "Contribute over at "
          [:a {:href "https://github.com/sharingio/pair"} "GitHub"]
          " and "
          [:a {:href "https://gitlab.com/sharingio/pair"} "GitLab"]
          "."]]
        [:p "To use sharing.io, you must be a public member of a permitted github org."]
        ]
       )]]
   user))

(defn new-box-form
  [{:keys [fullname email username admin-member]}]
  (form/form-to {:id "new-box" :onsubmit "
                                         var node = document.createElement('h3')
                                         var textnode = document.createTextNode('Your instance is now being created')
                                         node.appendChild(textnode)
                                         document.querySelector('form#new-box').appendChild(node)
                                         document.querySelector('input#newInstanceSubmit').remove()
                                         "}
                [:post "/instances/new"]
   (util/anti-forgery-field)
   [:label {:for "type"} "Type"]
   (form/drop-down "type" '("Kubernetes")
                   "kubernetes")
   [:label {:for "facility"} "Facility"]
   (form/drop-down "facility" '("sjc1" "any") "sjc1")
   (when admin-member
     [:div.form-group
       [:label {:for "kubernetesNodeCount"} "Additional node count"]
       (form/drop-down "kubernetesNodeCount"
                      '("0" "1" "2" "3") "0")])
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
    [:p.helper "please add github usernames, separated by whitespace" [:b "(case sensistive)"]]]
   [:div.form-group
    [:label {:form "envvars"} "Environment Variables"]
    [:textarea {:name "envvars"
                :id "envvars"
                :placeholder "PAIR=sharing\nSHARE=pairing"}]
    [:p.helper "Add env vars as KEY=value, with each new variable on its own line."]]
   [:div.form-group
    [:label {:for "noGitHubToken"} "Share GitHub token to instance"]
    [:input {:name "noGitHubToken" :type :checkbox :id "noGitHubToken" :value "noGitHubToken" :checked "true"}]]
   (when admin-member
     [:div.form-group
      [:label {:form "name"} "Custom Name for Instance"]
      [:input {:name "name"
                  :id "name"
                  :placeholder "coolbox-123"}]
      [:p.helper "You can set a custom name for your box, which will be used in dns."]])
   [:input#newInstanceSubmit {:type :submit :value "launch"}]))

(defn new
  [user instances]
  (let [max-instance-limit (Integer. (or (System/getenv "MAX_INSTANCE_LIMIT") 1))]
    (layout
     [:main
      [:header
       [:h2 "Create a new Pairing Box"]]
      (if (or (:admin-member user) (< (count instances) max-instance-limit))
      (new-box-form user)
      [:div.warning
       [:h2.warning__title "Max Instance Reached"]
       [:p.warning__message "You are limited to "max-instance-limit" instances, and cannot create more."]
       [:p.warning__message "If you think this is an error, please contact an admin."]
       [:p.warning__message [:a {:href "/instances"}"See your current instances"]]])
      ;; This will set the timezone field to the timezone of the client browser.  If js disabled, timezone is Pacific/Auckland
      [:script "document.querySelector('input#timezone').value=(new Intl.DateTimeFormat).resolvedOptions().timeZone;"]]
     user)))

(defn envvars-box
  [{:keys [envvars]}]
  [:section#envvars
   [:h3 "Environment variables"]
  (if envvars
  (let [keyvalue (map #(first (vec %)) envvars)]
    (list
     [:details
      [:summary "Environment variables declared on launch"]
      [:table#envvar-table [:thead#envvar-table-headers [:tr [:td [:b "Key"]] [:td [:b "Value"]]]]
       (for [[key value] keyvalue]
         [:tr [:td (-> key name)][:td value]])]]))
     [:p "No environment variables declared"])])

(defn kubeconfig-box
  [{:keys [kubeconfig uid instance-id owner]}]
    [:section#kubeconfig
  (if (and kubeconfig owner)
    (list
     [:h3 "Kubeconfig available "]
     [:a#kc-dl {:href (str "https://" (env :canonical-url) "/public-instances/" uid "/" instance-id "/kubeconfig")
                :download (str instance-id "-kubeconfig")} "download"]
     [:p "you can attach to the cluster immediately with this command: "]
     (code-box "kc-command"
               (str
                "export KUBECONFIG=$(mktemp -t kubeconfig-XXXXX) ; curl -s "
                "https://" (env :canonical-url) "/public-instances/" uid "/" instance-id "/kubeconfig > \"$KUBECONFIG\""
                " ; kubectl -n " (clojure.string/lower-case owner) " exec -it environment-0 -- attach"))
     [:details
      [:summary "See Full Kubeconfig"]
      (code-box "kc" kubeconfig)])
     [:h3 "Kubeconfig not yet available"])])

(defn tmate
  [{:keys [tmate-ssh tmate-web]}]
  (if (or (= "Not ready to fetch tmate session" tmate-web) (empty? tmate-ssh))
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

(defn pair-ssh-instance
  [{:keys [instance-id]}]
  (if instance-id
    (list
     [:h3 "pair-ssh-instance "]
     [:p "connect with SSH, passing through the SSH agent (options with no args)"]
     (code-box "pair-ssh-instance-command" (str "pair-ssh-instance " instance-id))
     [:a#psi-dl {:href "https://raw.githubusercontent.com/sharingio/pair/master/hack/pair-ssh-instance"
                :download "pair-ssh-instance"} "download pair-ssh-instance"]

     )))

(defn status
  [{:keys [facility type phase sites dns cert noGitHubToken kubernetesNodeCount external-ips]}]
  [:section#status
   (if (empty? phase)
     [:h3#phase "Status: Unknown"]
     [:h3#phase "Status: " phase])
   [:p#type "Type: " type]
   [:p#kubernetesNodeCount "Node count: " kubernetesNodeCount]
   [:p#facility "Region: " facility]
   [:p#external-ips-title "External IPs: " ]
   [:ul#external-ips
    (for [ip (map :address external-ips)]
      [:li [:p ip]])]
   [:h3 "Sites Available"]
   [:ul#sites-available
    (if (> (count (filter (complement empty?) sites)) 0)
    (for [site sites]
      [:li [:a {:href site
                :target "_blank"
                :rel "noreferrer noopener"} site]])
    [:p "No sites available"]
    )]])

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
     (if (not (nil? age))
       [:em#age "Created by " [:a {:href (str "https://github.com/" owner)
                                   :target "_blank"
                                   :rel "noreferrer nofollower"} owner] " " age " ago"]
       [:em#age "Loading..."])
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
    (pair-ssh-instance instance)
    (kubeconfig-box instance)
    (envvars-box instance)]
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
  [{:keys [instance-id phase age owner]}]
  [:li.instance [:a {:href (str "/instances/id/"instance-id)}
        instance-id] [:em.phase phase]
   (if (not (nil? age))
     [:p.age "Created by " [:a {:href (str "https://github.com/" owner)
                                :target "_blank"
                                :rel "noreferrer nofollower"} owner] " " age " ago"]
     [:em#age "Loading..."])])

(defn all-instances
  [instances {:keys [username admin-member] :as user}]
  (let [[owner rest] ((juxt filter remove) #(= (:owner %) username) instances)
        [guest other] ((juxt filter remove) #(some #{username} (:guests %)) rest)
        [countTotal] (str (count instances))
        [countOwner] (str (count owner))
        [countGuest] (str (count guest))
        [countOther] (str (count other))]
    (layout
     [:main#all-instances
      [:header
       [:h2 "Instances"]]
      [:article
       (if (= (str countTotal) "0")
         [:div
          [:p.instanceCountMessage "No instances found"]]
         [:div
          (when owner
            [:section#owner
             [:h3 "Created by You"]
             (if (= (str countOwner) "0")
               [:div
                [:p.instanceCountMessage "No instances created by you"]]
               [:div
                [:ul
                 (for [instance owner]
                   (instance-li instance))]
                [:p.instanceCount (str countOwner " instance(s)")]])])
          (when guest
            [:section#guest
             [:h3 "Shared with You"]
             (if (= (str countGuest) "0")
               [:div
                [:p.instanceCountMessage "No instances shared with you"]]
               [:div
                [:ul
                 (for [instance guest]
                   (instance-li instance))]
                [:p.instanceCount (str countGuest " instance(s)")]])])
          (when (and admin-member other)
            [:section#admin
             [:h3 "All Other Instances"]
             (if (= (str countOther) "0")
               [:div
                [:p.instanceCountMessage "No instances active"]]
               [:div
                [:ul
                 (for [instance other]
                   (instance-li instance))
                 [:p.instanceCount (str countOther " instance(s)")]]])])])]]
     user)))
