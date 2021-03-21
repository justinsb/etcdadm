Notes on how to sync the repos:

# Check out a clean copy
cd ~/k8s/src/kope.io
git clone https://github.com/kopeio/etcd-manager etcd-manager-rewrite
cd etcd-manager-rewrite

# Move to a subdirectory
git filter-repo --to-subdirectory-filter etcd-manager 

# Update some email addresses to keep the cla bot happy
cat > ~/etcd-manager-mailmap << EOF
<redacted@redacted.com> <redacted.redacted@redacted.com> 
EOF
git filter-repo --mailmap ~/etcd-manager-mailmap

# Fix up some of the commit messages with forbidden things (@mentions and closing bugs)
git filter-repo --commit-callback '
msg = commit.message.decode("utf-8")
msg = msg.replace("@granular", "granular")
msg = msg.replace("Fix #274", "Issue #274")
msg = msg.replace("Fixes #9730", "Issue #9730")
commit.message = msg.encode("utf-8")
' --refs HEAD

# Merge into target repo
cd ~/k8s/src/sigs.k8s.io/etcdadm
git remote add etcd-manager-rewrite ~/k8s/src/kope.io/etcd-manager-rewrite
git checkout -b add_etcd_manager
git reset --hard origin/master && git fetch etcd-manager-rewrite && git merge etcd-manager-rewrite/master --allow-unrelated-histories



# Rewrite go.mod and imports
find etcd-manager/ -type f | xargs sed -i -e 's@kope.io/etcd-manager@sigs.k8s.io/etcdadm/etcd-manager@g'
git add etcd-manager/
git commit -m "Rewrite kope.io/etcd-manager to sigs.k8s.io/etcdadm/etcd-manager"




## Incremental update

cd ~/k8s/src/kope.io
rm -rf etcd-manager-rewrite
git clone https://github.com/kopeio/etcd-manager etcd-manager-rewrite
cd etcd-manager-rewrite

git filter-repo --to-subdirectory-filter etcd-manager 

# Rewrite go.mod and imports
cat > /tmp/replacements <<EOF
kope.io/etcd-manager==>sigs.k8s.io/etcdadm/etcd-manager
EOF
git filter-repo --replace-text /tmp/replacements


cd ~/k8s/src/sigs.k8s.io/etcdadm
git fetch etcd-manager-rewrite

git log etcd-manager-rewrite/master


#git co master
#git rebase --onto update_etcd_manager_at_20201119 8c5aea6a 8d22adf2
#git co update_etcd_manager_at_20201119

git co master
git rebase -Xours --onto origin/master 86b2fdd5621b540cfcd6bf639a93c8cab47e1f18 ef9aacc6a209af864503b9ccc3f1e6622cc2bfa6
git co -b update_etcd_manager_at_20210321

git push ${USER}
hub pull-request -m "Update with latest etcd-manager changes" 