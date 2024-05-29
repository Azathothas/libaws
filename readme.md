
### explore the cli

```bash
>> libaws -h | grep ensure | head

codecommit-ensure             - ensure a codecommit repository
dynamodb-ensure               - ensure a dynamodb table
ec2-ensure-keypair            - ensure a keypair
ec2-ensure-sg                 - ensure a sg
ecr-ensure                    - ensure ecr image
iam-ensure-ec2-spot-roles     - ensure iam ec2 spot roles that are needed to use ec2 spot
iam-ensure-instance-profile   - ensure an iam instance-profile
iam-ensure-role               - ensure an iam role
iam-ensure-user-api           - ensure an iam user with api key
iam-ensure-user-login         - ensure an iam user with login
```

### explore a cli entrypoint

```bash
>> libaws s3-ensure -h

ensure a s3 bucket

example:
 - libaws s3-ensure test-bucket acl=public versioning=true

optional attrs:
 - acl=VALUE        (values = public | private, default = private)
 - versioning=VALUE (values = true | false,     default = false)
 - metrics=VALUE    (values = true | false,     default = true)
 - cors=VALUE       (values = true | false,     default = false)
 - ttldays=VALUE    (values = 0 | n,            default = 0)

setting 'cors=true' uses '*' for allowed origins. to specify one or more explicit origins, do this instead:
 - corsorigin=http://localhost:8080
 - corsorigin=https://example.com

Usage: s3-ensure [--preview] NAME [ATTR [ATTR ...]]

Positional arguments:
  NAME
  ATTR

Options:
  --preview, -p
  --help, -h             display this help and exit
```