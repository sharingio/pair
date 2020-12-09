(defn nav
  [title subtitle]
  [:nav
   [:a.title {:href "/"} title [:sup subtitle]]])
