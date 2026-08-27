# Upload package, create and deploy release

```
octopus package upload AwesomeWebsite.1.0.0.zip --no-prompt
octopus release create --project "Deploy Awesome Website" --package "AwesomeWebSite:1.0.0" --version 1.0.0 --channel Default --no-prompt
octopus release deploy --project "Deploy Awesome Website" --version 1.0.0 --environment "development" --no-prompt
octopus release deploy --project "Deploy Awesome Website" --version 1.0.0 --environment "test" --no-prompt
```

# Create tenant and deployment target

```
tenantName="Valley Veterinary Clinic"
echo "Creating new tenant, $tenantName"
octopus tenant create --name '$tenantName' --tag 'Importance/VIP' --tag 'Azure Region/West US 2' --no-prompt

webappName=$(sed 's/ /-/g' <<< "$tenantName" | tr '[:upper:]' '[:lower:]') # replace spaces and lowercase the name
echo "Creating new Azure Web App, $webappName"
az webapp create -g ClinicWebAppResourceGroup -p clinic-app-svc-plan -n $webappName --runtime DOTNETCORE:6.0 -o none

targetName='$tenantName web app'
echo "Creating new deployment target, $targetName"
octopus deployment-target azure-web-app create --name '$targetName'  --web-app $webappName --resource-group ClinicWebAppResourceGroup --tenanted-mode=tenanted --tenant '$tenantName' --environment 'Test'  --account AzureAccount --role vet-clinic-web-app --no-prompt

echo "Waiting for '$targetName' to be healthy"
status=`octopus deployment-target view "$targetName" | grep 'Health status'`
while true; do
    echo $status
    if [[ $status =~ 'Healthy' ]]; then
        break
    fi
    sleep 1
    status=`octopus deployment-target view "$targetName" | grep 'Health status'`
done

version='1.0.1'
projectName='Vet Clinic Web App'
environment='Test'
echo "Deploying '$projectName' version $version to '$environment" for '$tenantName'"
octopus release deploy --project "$projectName" --version $version --environment "$environment" --tenant "$tenantName" --no-prompt | octopus task wait
```

# Bulk adding tenants to project

From a static list:

```
filename="tenant-list.txt"
while read t; do
    octopus tenant connect --tenant "$t" --project 'New Awesome Project' --environment 'Test' --environment 'Production' --enable-tenant-deployments --no-prompt
done < "$filename"
```

From list filtered by tag:

```
octopus tenant list -f json | jq --raw-output '.[] | select (.TenantTags[]? | contains("Importance/VIP")) | .Name' | while read t; do
  octopus tenant connect --tenant $t --project 'New Awesome Project' --environment 'Test' --environment 'Prod' --enable-tenant-deployments --no-prompt
done
```

# Creating a new tenant, linked to an existing project with variables

```
name='Mountain Vet Clinic'
abrev='mtn'
octopus tenant create --name "$name" --no-prompt
octopus tenant connect --tenant "$name" --project "Vet Clinic" --environment Staging --environment Production --no-prompt
octopus tenant variable update --tenant "$name" --library-variable-set "Tenant shared" --name "Tenant.Abbreviation" --value "$abrev" --no-prompt
octopus tenant variable update --tenant "$name" --project "Vet Clinic" --name "Tenant.Database.Name" --environment "Staging" --value "Staging$abrev" --no-prompt
octopus tenant variable update --tenant "$name" --project "Vet Clinic" --name "Tenant.Database.Name" --environment "Production" --value "$abrev" --no-prompt
octopus tenant variable update --tenant "$name" --project "Vet Clinic" --name "Tenant.Azure.ServicePlan.Sku.Code" --environment "Staging" --value "B1" --no-prompt
octopus tenant variable update --tenant "$name" --project "Vet Clinic" --name "Tenant.Azure.ServicePlan.Sku.Code" --environment "Production" --value "B4ms" --no-prompt
```

# List all versions of all packages

```
octopus package list -f basic | while read p; do
    echo $p
    octopus package versions --package $p
    echo '\n'
done
```

# List the names of machines with a specific role

```
octopus deployment-target list -f json | jq --raw-output '.[] | select (.Roles[]? | contains("web server")) | .Name'
```

# Register an SSH endpoint

```
localIp=$(ifconfig eth0 | grep 'inet addr:' | cut -d: -f2 | awk '{ print $1}')
fingerprint=$(ssh-keygen -E md5 -lf /etc/ssh/ssh_host_rsa_key.pub | cut -d' ' -f2 | cut -d: -f2- | awk '{ print $1}')
monoExists=$(command -v mono)
if [ $monoExists ]
then
  octopus deployment-target ssh create --account "TheAccount" --name "MySshTargetName" --host $localIp --fingerprint $fingerprint --role linux --runtime mono --no-prompt
else
  octopus deployment-target ssh create --account "TheAccount" --name "MySshTargetName" --host $localIp --fingerprint $fingerprint --role linux --runtime self-contained --platform linux-x64 --no-prompt
fi
```

# Create deployment target with target tags

Target tag sets provide organized tagging with validation. Use `--tag` with canonical format (TagSetName/TagName):

```
octopus deployment-target ssh create \
  --account "TheAccount" \
  --name "MySshTargetName" \
  --host $localIp \
  --fingerprint $fingerprint \
  --environment Production \
  --tag "[System] Target Tags/web-server" \
  --tag Region/US-East \
  --runtime mono \
  --no-prompt
```

Note: The `--role` flag continues to work for backwards compatibility but will be deprecated in favor of `--tag` once target tag sets are widely adopted.

# Bulk deleting releases by created date

This example will delete all releases created before 2AM 6 Dec 2022 UTC
`jq` source: [Sebs IT Blog](https://megamorf.gitlab.io/cheat-sheets/jq/#select-item-in-time-range)

```
octopus release list -f json -p schedule-script | jq --arg date '2022-12-06T02:00' --raw-output '.[] | select(.Assembled | . < $date) | .Version' | while read t; do
  octopus release delete --project 'New Awesome Project' --version $t --no-prompt
done
```

# Create a project with Config as Code enabled

```
octopus project create --name 'Project 54' --group 'Default Project Group' --lifecycle 'Default Lifecycle' --no-prompt
octopus project convert --project 'Project 54' \
  --git-credential-store 'library' \
  --git-base-path '.octopus' \
  --git-url 'https://github.com/user/project-54-cac.git' \
  --git-branch 'main' \
  --git-initial-commit 'Initial commit of deployment process' \
  --git-credentials 'git-creds' \
  --git-initial-commit-branch 'initial-project-54' \
  --git-protected-default-branch \
  --no-prompt
```

An existing project can be converted to Config As Code using the `convert` command

# Deploy a release and wait for completion

```
octopus release deploy \
  --project 'New Awesome Project' \
  --version '0.0.4' \
  --environment 'test' \
  --tenant-tag 'customer type/early adopter' \
  --no-prompt \
  --output-format basic \
  | octopus task wait
```

Note: the `release deploy` command requires the `--output-format basic` flag to be able to pipe the server tasks Ids to the `task wait` command.

# View all values for a project variable

```
octopus project variables view BlueGreenTarget
```

# Set project variable prior to creating a release

In this example the `Id` represents the specific value for the variable `BlueGreenTarget` that has been scoped to the production environment.
The Id can be obtained with the `project variables view` command.

```
value=`octopus project variables view BlueGreenTarget --project "Random Quotes" --id d8527596-6fa2-4394-94e1-07942d3d0202 | grep Value`
if [[ $value =~ 'Blue' ]]; then
    value="Green"
else
    value="Blue"
fi
octopus project variables update BlueGreenTarget --project "Random Quotes" --id d8527596-6fa2-4394-94e1-07942d3d0202 --name "" --value $value --no-prompt
octopus release create --version 1.0.1 --project "Random Quotes" --no-prompt
```

# Install the Octopus Argo CD gateway

The gateway connects an Argo CD instance to Octopus. Run it with no arguments and the CLI reads
what it can from your cluster and from Octopus, asking only for the things it cannot work out:

```
octopus kubernetes gateway install
```

It discovers which namespace Argo CD is in, its in-cluster address, and whether it is serving
TLS; derives the install namespace and Helm release name from the name you give it; and takes
the Octopus server, space, and credentials from your existing login.

# Preview an Argo CD gateway install without changing anything

```
octopus kubernetes gateway install --name production --environment Production --dry-run
```

`--dry-run` renders the manifests Helm would apply and skips the connectivity checks that need
to run a pod. Add `-o values.yaml` to also write out the resolved Helm values.

# Install the Argo CD gateway unattended

```
octopus kubernetes gateway install \
  --name production \
  --environment Production \
  --argocd-token "$ARGOCD_TOKEN" \
  --no-prompt
```

Octopus registers the gateway itself before installing the chart, and puts only the gateway's
own credential into the cluster — your Octopus API key is never stored there. If the install
fails, the registration is removed again so you are not left with a gateway that never connects.

The Argo CD token is written to a Kubernetes Secret and referenced from the chart, so it does
not appear in the Helm release values or in a file written by `-o`. Pass `--inline-secrets` if
you would rather have it in the values.

# Let the CLI create the Argo CD account it needs

Octopus authenticates to Argo CD as a dedicated account, which normally means editing
`argocd-cm` and `argocd-rbac-cm` by hand and then running `argocd account generate-token`.
`--configure-argocd-account` does all three:

```
octopus kubernetes gateway install \
  --name production \
  --environment Production \
  --configure-argocd-account \
  --no-prompt
```

Interactively, the CLI shows you exactly which ConfigMap entries it would add and asks before
applying them. Add `--allow-sync=false` if Octopus should only observe Argo CD applications
rather than sync them.

# Install the Argo CD gateway against AWS managed Argo CD (EKS capability)

The [EKS capability for Argo CD](https://octopus.com/docs/argo-cd/instances/aws-managed-argo-cd)
runs Argo CD in the AWS control plane rather than on your nodes, so there is nothing in the
cluster to discover. Point the CLI at an EKS context and it works this out for you: it reads the
cluster name and region from your kubeconfig, asks AWS for the Argo CD capability endpoint, and
switches on the settings that mode needs — gRPC-Web (AWS's load balancer does not support
HTTP/2) and full TLS verification (AWS uses a publicly trusted certificate).

```
octopus kubernetes gateway install --kube-context arn:aws:eks:ap-southeast-2:123456789012:cluster/my-cluster
```

AWS caps Argo CD account tokens at 12 hours, so managed instances authenticate with project role
tokens instead — one per Argo CD project. Interactively, the CLI lists the projects in your
cluster, offers to add the `octopus` role with the right policies to the ones you pick, and links
you to Argo CD to generate each token. AWS signs those tokens in its own control plane, so that
last step cannot be automated.

Each token says which project it belongs to, so you only pass the token:

```
octopus kubernetes gateway install \
  --name eks-production \
  --environment Production \
  --argocd-server-grpc-url grpc://abcd1234.eks-capabilities.ap-southeast-2.amazonaws.com \
  --argocd-project-token "$DEFAULT_TOKEN" \
  --argocd-project-token "$TEAM_A_TOKEN" \
  --no-prompt
```

Use the project name `octo-gateway-unscoped` for a token to fall back on for Argo CD calls that
are not project-scoped. If your Argo CD API is not served at the root, add
`--argocd-grpc-web-root-path /argo/api`.
