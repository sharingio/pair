#!/bin/python

yaml = helm(
    'charts/sharingio-pair',
    name='sharingio-pair',
    namespace='sharingio-pair',
    set=[
        'sessionSecret=pairpairpairpair',
        'githubOAuth.id=' + os.getenv("SHARINGIO_PAIR_GITHUB_OAUTH_ID"),
        'githubOAuth.secret=' + os.getenv("SHARINGIO_PAIR_GITHUB_SECRET"),
        'equinixMetal.projectID=' + os.getenv("SHARINGIO_PAIR_EQUINIXMETAL_PROJECTID"),
        'ingress.enabled=true',
        'ingress.hosts[0].host=' + os.getenv("SHARINGIO_PAIR_HOST"),
        'ingress.hosts[0].paths[0]=/',
        'ingress.certmanager.enabled=true'
    ]
  )
k8s_yaml(yaml)

# if using a pair instance
if os.getenv('SHARINGIO_PAIR_NAME'):
    custom_build('registry.gitlab.com/sharingio/pair/client', 'docker build -f apps/client/Dockerfile -t $EXPECTED_REF apps/client', ['apps/client'], disable_push=True)
    custom_build('registry.gitlab.com/sharingio/pair/clusterapimanager', 'docker build -f apps/cluster-api-manager/Dockerfile -t $EXPECTED_REF apps/cluster-api-manager', ['apps/cluster-api-manager'], disable_push=True)
# standard
else:
    docker_build('registry.gitlab.com/sharingio/pair/client', 'apps/client', dockerfile="apps/client/Dockerfile")
    docker_build('registry.gitlab.com/sharingio/pair/clusterapimanager', 'apps/cluster-api-manager', dockerfile="apps/cluster-api-manager/Dockerfile")

# disallow production clusters
allow_k8s_contexts('in-cluster')
