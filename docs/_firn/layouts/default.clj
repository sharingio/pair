(defn default
  [{:keys [render partials title]}]
  (let [{:keys [head header nav]} partials]
    (head
     [:body
      (nav "Sharing.io" "docs")
      [:main
       [:aside.sidebar
        (render :sitemap)]
       [:article.content
        ;; [:div (render :toc)] ;; Optional; add a table of contents
        [:h1 title]
        [:div (render :file)]]]])))
