package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/sts"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"gopkg.in/ini.v1"
)

// LoadIni will load a filename as an INI file, as well as
// a slice of the config sections
func LoadIni(filename string) (*ini.File, []*ini.Section, error) {
	iniFile, err := ini.Load(filename)
	if err != nil {
		return nil, nil, err
	}
	sections := iniFile.Sections()
	return iniFile, sections, nil
}

// SetCreds will populate the aws credentials file(passed as INI)
// with a new 'default' section and updated STS credentials
func SetCreds(filename string, creds *sts.Credentials, f *ini.File) {
	section, err := f.NewSection("default")
	if err != nil {
		fmt.Println(err)
	}

	// Populate the section with the temp creds
	fmt.Println("Setting temporary credentials in your file")
	section.NewKey("aws_access_key_id", *creds.AccessKeyId)
	section.NewKey("aws_secret_access_key", *creds.SecretAccessKey)
	section.NewKey("aws_session_token", *creds.SessionToken)

	f.SaveTo(filename)
}

// Authenticate with the AWS CLI
func Authenticate(profile, mfa, token, credfile, conffile string, s *sts.STS, conf, cred *ini.File) {
	activeProf := conf.Section(profile)
	// This handles authentication when a credential section is role-based instead of user-based
	if activeProf.HasKey("role_arn") {
		roleArn, _ := activeProf.GetKey("role_arn")
		res, err := s.GetSessionToken(&sts.GetSessionTokenInput{
			TokenCode:    aws.String(token),
			SerialNumber: aws.String(mfa),
		})

		if err != nil {
			log.Fatalln(err)
		}

		id := *res.Credentials.AccessKeyId
		secret := *res.Credentials.SecretAccessKey
		stoken := *res.Credentials.SessionToken

		sessConf := &aws.Config{
			Credentials: credentials.NewStaticCredentials(id, secret, stoken),
		}

		sess, err := session.NewSession(sessConf)
		if err != nil {
			fmt.Println(err)
		}

		svc := sts.New(sess)
		input := &sts.AssumeRoleInput{
			RoleArn:         aws.String(roleArn.String()),
			DurationSeconds: aws.Int64(3600),
			RoleSessionName: aws.String(profile + "-mfa-session"),
		}

		result, err := svc.AssumeRole(input)
		if err != nil {
			fmt.Println(err)
		}

		SetCreds(credfile, result.Credentials, cred)

		fmt.Println("Complete!")

	} else {
		result, err := s.GetSessionToken(&sts.GetSessionTokenInput{
			TokenCode:    aws.String(token),
			SerialNumber: aws.String(mfa),
		})

		if err != nil {
			log.Fatalln(err)
		}

		SetCreds(credfile, result.Credentials, cred)

		fmt.Println("Complete!")

	}
}

func main() {
	// Load the config and credential filenames
	credfile := defaults.SharedCredentialsFilename()
	configfile := defaults.SharedConfigFilename()

	// Clear any existing env vars to avoid issues
	awsvars := []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_KEY_ID", "AWS_SESSION_TOKEN"}

	fmt.Println("Resetting AWS Environment variables...")
	fmt.Println("===================================")
	for _, v := range awsvars {
		err := os.Setenv(v, "")
		if err != nil {
			fmt.Println(err)
		}
	}

	// Load the credentials and config files as ini files
	awscreds, credprofiles, err := LoadIni(credfile)
	awsconfig, _, err := LoadIni(configfile)

	var profiles []string

	for _, s := range credprofiles {
		if !(strings.Contains(strings.ToLower(s.Name()), "default")) {
			profiles = append(profiles, s.Name())
		}
	}

	fmt.Println("Choose which AWS profile to authenticate with:")
	for i, p := range profiles {
		fmt.Printf("%d: %s\n", i+1, p)
	}

	chooseProfile := func() string {
		var profile string
		for {
			reader := bufio.NewReader(os.Stdin)
			text, _ := reader.ReadString('\n')
			text = strings.Replace(text, "\n", "", -1)

			i, err := strconv.Atoi(text)
			if err != nil {
				fmt.Println(err)
				continue
			}

			if i >= 1 && i <= len(profiles) {
				profile = profiles[i-1]
				break
			}
			fmt.Println("Please choose one of the available profiles")
			continue
		}
		return profile
	}

	profile := chooseProfile()

	// Read in hard-coded config/credentials(needed for STS handoff)
	conf := &aws.Config{
		Credentials: credentials.NewSharedCredentials(credfile, profile),
	}

	// Create new session
	sess, err := session.NewSession(conf)

	// Create session with STS service for getting the initial caller identity
	// and the eventual auth token
	_sts := sts.New(sess)

	// Get ARN of profile user
	arn, err := _sts.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		fmt.Println(err)
	}

	// Create session with IAM service to get MFA device for chosen profile
	_iam := iam.New(sess)

	mfas, err := _iam.ListVirtualMFADevices(&iam.ListVirtualMFADevicesInput{
		AssignmentStatus: aws.String("Assigned"),
	})

	var mfa string

	for _, device := range mfas.VirtualMFADevices {
		if *device.User.Arn == *arn.Arn {
			mfa = *device.SerialNumber
		}
	}


	// Get MFA token from user
	token, err := stscreds.StdinTokenProvider()

	if err != nil {
		fmt.Println(err)
	}

	// Use regex to validate a 6 digit number and catch
	// malformed input without wasting an API call

	tokenmatch, _ := regexp.MatchString(`^\d{6}$`, token)

	for !tokenmatch {
		fmt.Println("Please enter a valid 6 digit token")

		token, err = stscreds.StdinTokenProvider()

		if err != nil {
			fmt.Println(err)
		}
	}


	// Perform authentication
	Authenticate(profile, mfa, token, credfile, configfile, _sts, awsconfig, awscreds)
}
