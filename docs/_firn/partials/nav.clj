(defn nav
  [title subtitle]
  [:nav#top
   [:a.title {:href "/"} title [:sup subtitle]]
   [:div.ext-links
    [:a {:href "https://sharing.io"
         :target "_blank"
         :rel "noreferrer noopener"}
     "home"]
    [:a {:href "https://github.com/sharingio/pair"
         :target "_blank"
         :rel "noreferrer noopener"}
     "source"]]])
