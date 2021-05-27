# aws-mfa

aws-mfa is a simple CLI application written in Golang for handling AWS CLI MFA authentication.

## How it works

The application uses the [AWS STS API](https://docs.aws.amazon.com/STS/latest/APIReference/welcome.html) and your local credentials file to request temporary AWS credentials, authenticated via Multifactor Authentication(MFA).

## Prerequisites

It is assumed, at a minimum, you have access to an AWS account. You will also need:

- An IAM user or assumable role with permissions to use the STS service.
- An MFA device configured for your respective identity, capable of generating TOTP tokens.
- Configuration and credentials files in the standard locations(typically `$HOME/.aws/config` and `$HOME/.aws/credentials` respectively), with at least one account profile.

Although it's not technically "required", installing the [AWS CLI](https://aws.amazon.com/cli/) is highly recommended, as it is the most common way to create the required configuration directory and files, and ultimately provides the quickest way to validate that your credentials are valid.

## Quickstart

1. Download the latest release from the [releases page](https://github.com/codevbus/aws-mfa/releases).
2. Unzip the application to somewhere in your path, such as `/usr/local/bin`
3. Run `aws-mfa` and follow the instructions.
4. Test authentication by running `aws sts get-caller-identity`

## Credentials File Format

aws-mfa makes some assumptions about how your `$HOME/.aws/credentials` file is formatted, specifically around profile names. The easiest method is to name each profile something short and recognizable. An example credentials file for two separate AWS accounts, `dev` and `prod`:

``` ini
[dev]
aws_access_key_id     = <your_access_key>
aws_secret_access_key = <your_secret_key>

[prod]
aws_access_key_id     = <your_access_key> 
aws_secret_access_key = <your_secret_key> 
```

Note that there is *not* a profile named `default`.

If your current configuration includes a "default" profile, make a backup of your `$HOME/.aws` directory, and then rename the profile as described above. The next section explains the reasoning behind this.

## Ephemeral Credentials

The `aws` cli command, as well as most CLI utilities and applications that make calls to AWS and the AWS API are generally configured to expect AWS credentials in a few standard file locations, or as environment variables.

In the case of `aws-mfa`, we depend on the file method. In a credentials file, any profile named `default` will take precedence when making calls to the API. Once authentication completes successfull, `aws-mfa` dynamically sets a new `default` profile in `$HOME/.aws/credentials` containing temporary credentials.

Using the example file from above, here is what it would look like after a successful MFA authentication:

``` ini
[dev]
aws_access_key_id     = <your_access_key>
aws_secret_access_key = <your_secret_key>

[prod]
aws_access_key_id     = <your_access_key> 
aws_secret_access_key = <your_secret_key> 

[default]
aws_access_key_id     = <your_temporary_access_key>
aws_secret_access_key = <your_temporary_secret_key>
aws_session_token     = <your_temporary_session_token>
```

Note the extra key, `aws_session_token`, in the new `default` section.
