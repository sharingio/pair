(defn nav
  [title subtitle]
  [:header
   [:a {:href "/"} title [:sup subtitle]]])
