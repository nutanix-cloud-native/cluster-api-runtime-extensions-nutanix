+++
title = "Users"
+++

Configure users for all machines in the cluster, the user's superuser capabilities using `sudo` user specifications, and
the login authentication mechanism.

> - SSH _authorized keys_ are just public SSH keys that are used to authenticate a login. See the [SSH man
>   page](https://www.man7.org/linux/man-pages/man8/sshd.8.html#AUTHORIZED_KEYS_FILE_FORMAT) for more information.
>
> - For information on sudo user specifications, see the [sudo
>   documentation](https://www.sudo.ws/docs/man/sudoers.man/#User_specification).
>
> - Local password authentication is disabled for the user by default. It is enabled only when a hashed password is
>   provided.

## Examples

### Admin user with SSH public key login

Creates a user with the name `admin`, grants the user the ability to run any command as the superuser, and allows you to
login via SSH using the username and private key corresponding to the authorized public key.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    variables:
      - name: clusterConfig
        value:
          users:
            - name: username
              sshAuthorizedKeys:
                - "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAua0lo8BiGWgvIiDCKnQDKL5uERHfnehm0ns5CEJpJw optionalcomment"
              sudo: "ALL=(ALL) NOPASSWD:ALL"
```

### Admin user with serial console password login

Creates a user with the name `admin,` grants the user the ability to run any command as the superuser, and allows you to
login via serial console using the username and password.

> Note that this does not allow you to login via SSH using the username and password; in most cases, you must also
> configure the SSH server to allow password authentication.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    variables:
      - name: clusterConfig
        value:
          users:
            - name: admin
              hashedPassword: "$y$j9T$UraH8eN4XvapXBmmSaUrP0$Nyxdf1cJDGZcp0WDKu.CFHprrkPG4ubirqSqiD43Ix3"
              sudo: "ALL=(ALL) NOPASSWD:ALL"
```
