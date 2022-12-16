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
octopus deployment-target list -f json | jq --raw-output | select (.Roles[]? | contains("web server")) | .Name'
```

# Register an SSH endpoint

```
localIp=$(ifconfig eth0 | grep 'inet addr:' | cut -d: -f2 | awk '{ print $1}')
fingerprint=$(ssh-keygen -E md5 -lf /etc/ssh/ssh_host_rsa_key.pub | cut -d' ' -f2 | cut -d: -f2- | awk '{ print $1}')
monoExists=$(command -v mono)
if [ $monoExists ]
then
  octopus deployment-target ssh create --account "TheAccount" --name "MySshTargetName" --host $localIp --fingerprint $fingerprint--role linux --runtime mono --no-prompt
else
  octopus deployment-target ssh create --account "TheAccount" --name "MySshTargetName" --host $localIp --fingerprint $fingerprint--role linux --runtime self-contained --platform linux-x64 --no-prompt
fi
```

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