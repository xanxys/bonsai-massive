# Preparation
* docker
** Don't forget to add your user to `docker` group (`./start_bonsai.py` doesn't contain `sudo`)
* gcloud
* kubectl
** Need to get credential initially; see [this doc](https://cloud.google.com/sdk/gcloud/reference/container/clusters/get-credentials)

# Credentials
* Download the cloud project key of the prod account to somewhere local (a json file)
* `./start_bonsai.py --key <path to cred> --remote --env staging`
