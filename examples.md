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
status=`./octopus deployment-target view "$targetName" | grep 'Health status'`
while true; do
    echo $status
    if [[ $status =~ 'Healthy' ]]; then
        break
    fi
    sleep 1
    status=`./octopus deployment-target view "$targetName" | grep 'Health status'`
done

version="1.0.1"
projectName='Vet Clinic Web App'
environment="Test"
echo "Deploying '$projectName' version $version to '$environment" for '$tenantName'"
octopus release deploy --project "$projectName" --version $version --environment "$environment" --tenant "$tenantName" --no-prompt | octopus task wait
```
