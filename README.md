# gtokenserver

## Abstract

`gtokenserver` is a [google metadata server](https://cloud.google.com/compute/docs/storing-retrieving-metadata) emulator that provides access tokens.
`gtokenserver` allows Google Cloud SDK tools (gcloud, gsutil, bq) and applications using Google Cloud client libraries authenticate for Google Cloud Platform, instead of `gcloud auth login`, `gloud auth application-default login`, etc.


## Pros

* You can authenticate Google Cloud SDK tools (gcloud, gsutil, bq) and applications using Google Cloud client libraries in the same way.
    * `gcloud auth` requires you to use `gcloud auth login` (or `gcloud auth activate-service-account`) for Google Cloud SDK tools, and `gloud auth application-default login` (or the `GOOGLE_APPLICATION_CREDENTIALS` environment variable) for applications using Google Cloud client libraries.
* You can authenticate applications using Google Cloud client libraries with user accounts and service accounts in the same way.
    * You have to run `gloud auth application-default login` for user accounts, and you have to configure the `GOOGLE_APPLICATION_CREDENTIALS` environment variable for service accounts.
    * You have to launch `gtokenserver` in different ways, but you can lanunch applications requiring authentication in the same way.

## Usage

### With docker

1. Create a new network (`gcloud` for here):

    ```shell
    docker network create gcloud
    ```

2. Run `gtokenserver`:

    * For user accounts:

        1. Create a new volume (`gcloud-config` for here):

            ```shell
            docker volume create gcloud-config
            ```

        2. Run `gcloud auth application-default login`:

            ```shell
            docker run --rm -it -v gcloud-config:/gcloud-config -e CLOUDSDK_CONFIG=/gcloud-config \
                google/cloud-sdk:alpine gcloud auth application-default login
            ```

        3. Run `gtokenserver`:

            ```shell
            docker run -v gcloud-config:/gcloud-config -e CLOUDSDK_CONFIG=/gcloud-config \
                --network gcloud -d --rm --name gtokenserver ikedam/gtokenserver
            ```

    * For service accounts:

        1. Run `gtokenserver` with the private key json file :

            ```shell
            docker run -v "/path/to/service-account-private-key.json` -e GOOGLE_APPLICATION_CREDENTIALS=/key.json \
                --network gcloud -d --rm --name gtokenserver ikedam/gtokenserver
            ```

    * You may want to run with `--restart always` instead of `--rm` to have `gtokenserver` resident in your computer.

3. Run applications require authentication:

    * Google SDK tools:

        ```shell
        docker run --rm --network gcloud -e GCE_METADATA_ROOT=gtokenserver \
            google/cloud-sdk:alpine gcloud projects list
        ```

    * Applications using Google Cloud client libraries (Let's use [sops](https://github.com/mozilla/sops#22encrypting-using-gcp-kms) for example):

        ```shell
        cat test.yaml | \
            docker run --rm --network gcloud -e GCE_METADATA_HOST=gtokenserver -i \
            mozilla/sops:alpine --encrypt \
            --gcp-kms projects/my-project/locations/global/keyRings/sops/cryptoKeys/sops-key \
            --input-type yaml /dev/stdin
        ```

    * Be careful that Google SDK tools refers `GCE_METADATA_ROOT` but Google client libraries refers `GCE_METADATA_HOST`.

4. To stop gtokenserver:

    ```shell
    docker kill gtokenserver
    ```

### On the local machine

1. Run `gtokenserver`:

    * For user accounts:

        1. Run `gcloud auth application-default login`:

            ```shell
            gcloud auth application-default login
            ```

        2. Run `gtokenserver`:

            ```shell
            gtokenserver
            ```

            * It binds locahost:8080 by default. You can change the port with the `-p` option.

    * For service accounts:

        1. Run `gtokenserver` with the private key json file :

            ```shell
            GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account-private-key.json gtokenserver
            ```

2. Run applications require authentication:

    * Google SDK tools:

        ```shell
        GCE_METADATA_ROOT=localhost:8080 gcloud projects list
        ```

    * Applications using Google Cloud client libraries (Let's use [sops](https://github.com/mozilla/sops#22encrypting-using-gcp-kms) for example):

        ```shell
        GCE_METADATA_HOST=localhost:8080 sops --encrypt \
            --gcp-kms projects/my-project/locations/global/keyRings/sops/cryptoKeys/sops-key \
            test.yaml > test.enc.yaml
        ```

    * Be careful that Google SDK tools refers `GCE_METADATA_ROOT` but Google client libraries refers `GCE_METADATA_HOST`.


## Limitations

* `gtokenserver` doesn't provide all features of Google metadata servers. It's designed only to provide access token.
