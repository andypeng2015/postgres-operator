package main

/*
 Copyright 2018 - 2022 Crunchy Data Solutions, Inc.
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

import (
	"os"
	"strconv"
	"strings"

	"github.com/crunchydata/postgres-operator/internal/kubeapi"
	crv1 "github.com/crunchydata/postgres-operator/pkg/apis/crunchydata.com/v1"
	log "github.com/sirupsen/logrus"
)

const backrestCommand = "pgbackrest"

const backrestBackupCommand = `backup`
const backrestInfoCommand = `info`
const backrestStanzaCreateCommand = `stanza-create`
const containername = "database"
const repoTypeFlagS3 = "--repo1-type=s3"
const noRepoS3VerifyTLS = "--no-repo1-s3-verify-tls"

func main() {
	log.Info("pgo-backrest starts")

	debugFlag := os.Getenv("CRUNCHY_DEBUG")
	if debugFlag == "true" {
		log.SetLevel(log.DebugLevel)
		log.Debug("debug flag set to true")
	} else {
		log.Info("debug flag set to false")
	}

	Namespace := os.Getenv("NAMESPACE")
	log.Debugf("setting NAMESPACE to %s", Namespace)
	if Namespace == "" {
		log.Error("NAMESPACE env var not set")
		os.Exit(2)
	}

	COMMAND := os.Getenv("COMMAND")
	log.Debugf("setting COMMAND to %s", COMMAND)
	if COMMAND == "" {
		log.Error("COMMAND env var not set")
		os.Exit(2)
	}

	COMMAND_OPTS := os.Getenv("COMMAND_OPTS")
	log.Debugf("setting COMMAND_OPTS to %s", COMMAND_OPTS)

	PODNAME := os.Getenv("PODNAME")
	log.Debugf("setting PODNAME to %s", PODNAME)
	if PODNAME == "" {
		log.Error("PODNAME env var not set")
		os.Exit(2)
	}

	REPO_TYPE := os.Getenv("PGBACKREST_REPO_TYPE")
	log.Debugf("setting REPO_TYPE to %s", REPO_TYPE)

	// determine the setting of PGHA_PGBACKREST_LOCAL_S3_STORAGE
	// we will discard the error and treat the value as "false" if it is not
	// explicitly set
	PGHA_PGBACKREST_LOCAL_S3_STORAGE, _ := strconv.ParseBool(os.Getenv("PGHA_PGBACKREST_LOCAL_S3_STORAGE"))
	log.Debugf("setting PGHA_PGBACKREST_LOCAL_S3_STORAGE to %v", PGHA_PGBACKREST_LOCAL_S3_STORAGE)

	// parse the environment variable and store the appropriate boolean value
	// we will discard the error and treat the value as "false" if it is not
	// explicitly set
	PGHA_PGBACKREST_S3_VERIFY_TLS, _ := strconv.ParseBool(os.Getenv("PGHA_PGBACKREST_S3_VERIFY_TLS"))
	log.Debugf("setting PGHA_PGBACKREST_S3_VERIFY_TLS to %v", PGHA_PGBACKREST_S3_VERIFY_TLS)

	client, err := kubeapi.NewClient()
	if err != nil {
		panic(err)
	}

	bashcmd := make([]string, 1)
	bashcmd[0] = "bash"
	cmdStrs := make([]string, 0)

	switch COMMAND {
	case crv1.PgtaskBackrestStanzaCreate:
		log.Info("backrest stanza-create command requested")
		cmdStrs = append(cmdStrs, backrestCommand)
		cmdStrs = append(cmdStrs, backrestStanzaCreateCommand)
		cmdStrs = append(cmdStrs, COMMAND_OPTS)
	case crv1.PgtaskBackrestInfo:
		log.Info("backrest info command requested")
		cmdStrs = append(cmdStrs, backrestCommand)
		cmdStrs = append(cmdStrs, backrestInfoCommand)
		cmdStrs = append(cmdStrs, COMMAND_OPTS)
	case crv1.PgtaskBackrestBackup:
		log.Info("backrest backup command requested")
		cmdStrs = append(cmdStrs, backrestCommand)
		cmdStrs = append(cmdStrs, backrestBackupCommand)
		cmdStrs = append(cmdStrs, COMMAND_OPTS)
	default:
		log.Error("unsupported backup command specified " + COMMAND)
		os.Exit(2)
	}

	if PGHA_PGBACKREST_LOCAL_S3_STORAGE {
		firstCmd := cmdStrs
		cmdStrs = append(cmdStrs, "&&")
		cmdStrs = append(cmdStrs, strings.Join(firstCmd, " "))
		cmdStrs = append(cmdStrs, repoTypeFlagS3)
		// pass in the flag to disable TLS verification, if set
		// otherwise, maintain default behavior and verify TLS
		if !PGHA_PGBACKREST_S3_VERIFY_TLS {
			cmdStrs = append(cmdStrs, noRepoS3VerifyTLS)
		}
		log.Info("backrest command will be executed for both local and s3 storage")
	} else if REPO_TYPE == "s3" {
		cmdStrs = append(cmdStrs, repoTypeFlagS3)
		// pass in the flag to disable TLS verification, if set
		// otherwise, maintain default behavior and verify TLS
		if !PGHA_PGBACKREST_S3_VERIFY_TLS {
			cmdStrs = append(cmdStrs, noRepoS3VerifyTLS)
		}
		log.Info("s3 flag enabled for backrest command")
	}

	log.Infof("command to execute is [%s]", strings.Join(cmdStrs, " "))

	log.Infof("command is %s ", strings.Join(cmdStrs, " "))
	reader := strings.NewReader(strings.Join(cmdStrs, " "))
	output, stderr, err := kubeapi.ExecToPodThroughAPI(client.Config, client, bashcmd, containername, PODNAME, Namespace, reader)
	if err != nil {
		log.Info("output=[" + output + "]")
		log.Info("stderr=[" + stderr + "]")
		log.Error(err)
		os.Exit(2)
	}
	log.Info("output=[" + output + "]")
	log.Info("stderr=[" + stderr + "]")

	log.Info("pgo-backrest ends")

}
