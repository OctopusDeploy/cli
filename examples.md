# Upload package, create and deploy release

```
octopus package upload AwesomeWebsite.1.0.0.zip --no-prompt
octopus release create --project "Deploy Awesome Website" --package "AwesomeWebSite:1.0.0" --version 1.0.0 --channel Default --no-prompt
octopus release deploy --project "Deploy Awesome Website" --version 1.0.0 --environment "development" --no-prompt
octopus release deploy --project "Deploy Awesome Website" --version 1.0.0 --environment "test" --no-prompt
```
