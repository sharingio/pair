(defn head
  [body]
  [:html
   [:head
    [:meta {:charset "UTF-8"}]
    [:link {:href "https://fonts.gstatic.com"
            :rel "preconnect"}]
    [:link {:href "https://fonts.googleapis.com/css2?family=Manrope:wght@200;400;600;800&amp;display=swap"
            :rel "stylesheet"}]
    [:link {:rel "stylesheet" :href "/static/css/firn_base.css"}]
    [:link {:rel "stylesheet" :href "/static/css/ii_base.css"}]]
   body])
