# Preparation
* docker
 * Don't forget to add your user to `docker` group (`./start_bonsai.py` doesn't contain `sudo`)
* gcloud
* kubectl
 * Need to get credential initially; see [this doc](https://cloud.google.com/sdk/gcloud/reference/container/clusters/get-credentials)

# Credentials
* Download the cloud project key of the service account to somewhere local (a json file)
 * You need to renew it if you've lost it
* `./start_bonsai.py --key <path to cred> --remote --env staging`

With a incorrect key, it launches cluster, but /debug fails with error like this:
`private key should be a PEM or plain PKSC1 or PKCS8; parse error: asn1: syntax error: sequence truncated`.
