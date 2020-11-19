(ns client.db
  (:require [next.jdbc :as jdbc]
            [next.jdbc.result-set :as rs]
            [environ.core :refer [env]]
            [clojure.spec.alpha :as s]
            [clojure.spec.test.alpha :as test]))

(def db {:dbtype "postgresql"
            :dbname "pair"
            :host "postgres.pair"
            :user "pair"
            :password "pair"})

(def ds (jdbc/get-datasource db))

(defn add-user
  [{:keys [username fullname avatar email permitted-org-member]}]
  (jdbc/execute! db ["
insert into public.user
(username, full_name, avatar_url, email, permitted_org_member)
values(?,?,?,?,?)" username fullname avatar email permitted-org-member]))

(defn find-user
  "Returns info for existing user from db or nil"
  [username]
  (jdbc/execute-one! ds
   ["select username, full_name, email, permitted_org_member, avatar_url
       from public.user
      where username = ?" username]
   {:return-keys true :builder-fn rs/as-unqualified-lower-maps}))

(defn update-user
  [{:keys [username fullname avatar email permitted-org-member]}]
(jdbc/execute! ds ["
update public.user
   set (full_name, avatar_url, email, permitted_org_member)=(?, ?,?,?)
where username = ?" fullname avatar email permitted-org-member username]))

(defn find-instance
  "Returns info for existing instance from db or nil"
  [username project]
  (jdbc/execute-one! ds ["
select id, owner, instance_id, kubeconfig, tmate, project, facility, type, description,  status, phase, at
       from public.instance
      where owner = ?
        and project = ?
" username project]{:return-keys true :builder-fn rs/as-unqualified-lower-maps}))

(defn add-instance
  "Add minimal info for a new instance"
  [{:keys [owner project facility type instance-id status]}]
  (jdbc/execute! ds ["
insert into public.instance
(owner, project, facility, type, instance_id, status)
values(?,?,?,?,?,?)
" owner project facility type instance-id status]))

(defn new-instance
  "receive payload from packet, add entries to instance table and guest table"
  [payload]
  (add-instance payload))

(defn update-instance
  [{:keys [instance-id phase kubeconfig tmate]}]
  (jdbc/execute! ds ["
update instance
   set(phase, kubeconfig,tmate)=(?,?,?)
 where instance_id = ?
 returning instance_id, kubeconfig, phase, tmate, project, owner
" phase kubeconfig tmate instance-id]))

(defn create-user-table
  [ds]
  (jdbc/execute! ds ["
create table public.user
(
  username text not null unique primary key,
  full_name text,
  permitted_org_member boolean,
  avatar_url text,
  email text,
  data jsonb
)"]))

(defn create-instance-table
  [ds]
  (jdbc/execute! ds ["
create table public.instance
(
  id serial primary key,
  owner  text references public.user(username),
  project text not null,
  facility text,
  instance_id text,
  type  text,
  description text,
  status text,
  tmate text,
  ip text,
  kubeconfig text,
  at timestamp not null default current_timestamp
)"]))


(defn migrate
  [ds]
  (jdbc/with-transaction [tx ds]
    (create-user-table tx)
    (create-instance-table tx)))


(defn -main
  []
  (migrate ds)
  (println "migrations applied"))
