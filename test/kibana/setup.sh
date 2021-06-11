# ELASTICSEARCH_USER_ID=${ELASTICSEARCH_USER_ID:-1000}

# userdel elasticsearch &>/dev/null
# adduser -u $ELASTICSEARCH_USER_ID elasticsearch -D

# datadir=$(cat /etc/rancher-conf/elasticsearch/elasticsearch.yml | grep path.data | awk -F: '{ print $2 }')
# if [[ "$datadir" != "" ]]; then
#   mkdir -p $datadir &>/dev/null
#   chown -R elasticsearch:elasticsearch $datadir
# fi

# chown -R elasticsearch:elasticsearch /etc/rancher-conf/elasticsearch
