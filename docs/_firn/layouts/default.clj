(defn default
  [{:keys [render partials title]}]
  (let [{:keys [head header nav]} partials]
    (head
     [:body
      (nav "Sharing.io" "docs")
      [:main
       [:aside.sidebar
        (render :sitemap {:sort-by :firn-order})]
       [:article.content
        ;; [:div (render :toc)] ;; Optional; add a table of contents
        [:h1 title]
        [:div (render :file)]]]])))
