/*
Copyright (C) 2021-2023, Kubefirst

This program is licensed under MIT.
See the LICENSE file for more details.
*/
package k3d

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/kubefirst/runtime/pkg"
	"github.com/kubefirst/runtime/pkg/gitClient"
	cp "github.com/otiai10/copy"
	"github.com/rs/zerolog/log"
)

func AdjustGitopsRepo(cloudProvider, clusterName, clusterType, gitopsRepoDir, gitopsRepoName, gitProvider, k1Dir string, removeAtlantis bool) error {

	//* clean up all other platforms
	for _, platform := range pkg.SupportedPlatforms {
		if platform != fmt.Sprintf("%s-%s", CloudProvider, gitProvider) {
			os.RemoveAll(gitopsRepoDir + "/" + platform)
		}
	}

	//* copy options
	opt := cp.Options{
		Skip: func(src string) (bool, error) {
			if strings.HasSuffix(src, ".git") {
				return true, nil
			} else if strings.Index(src, "/.terraform") > 0 {
				return true, nil
			}
			//Add more stuff to be ignored here
			return false, nil

		},
	}

	//* copy $cloudProvider-$gitProvider/* $HOME/.k1/gitops/
	driverContent := fmt.Sprintf("%s/%s-%s/", gitopsRepoDir, CloudProvider, gitProvider)
	err := cp.Copy(driverContent, gitopsRepoDir, opt)
	if err != nil {
		log.Info().Msgf("Error populating gitops repository with driver content: %s. error: %s", fmt.Sprintf("%s-%s", CloudProvider, gitProvider), err.Error())
		return err
	}
	os.RemoveAll(driverContent)

	//* copy $HOME/.k1/gitops/cluster-types/${clusterType}/* $HOME/.k1/gitops/registry/${clusterName}
	clusterContent := fmt.Sprintf("%s/cluster-types/%s", gitopsRepoDir, clusterType)
	err = cp.Copy(clusterContent, fmt.Sprintf("%s/registry/%s", gitopsRepoDir, clusterName), opt)
	if err != nil {
		log.Info().Msgf("Error populating cluster content with %s. error: %s", clusterContent, err.Error())
		return err
	}
	os.RemoveAll(fmt.Sprintf("%s/cluster-types", gitopsRepoDir))
	os.RemoveAll(fmt.Sprintf("%s/services", gitopsRepoDir))

	registryLocation := fmt.Sprintf("%s/registry/%s", gitopsRepoDir, clusterName)
	if pkg.LocalhostARCH == "arm64" && cloudProvider == CloudProvider {
		amdConsoleFileLocation := fmt.Sprintf("%s/components/kubefirst/console.yaml", registryLocation)
		os.Remove(amdConsoleFileLocation)
	} else {
		armConsoleFileLocation := fmt.Sprintf("%s/components/kubefirst/console-arm.yaml", registryLocation)
		os.Remove(armConsoleFileLocation)
	}

	if removeAtlantis {
		atlantisRegistryFileLocation := fmt.Sprintf("%s/atlantis.yaml", registryLocation)
		os.Remove(atlantisRegistryFileLocation)
	}

	path := fmt.Sprintf("%s/%s", gitopsRepoDir, "terraform/github/repos.tf")
	tmplpath := fmt.Sprintf("%s/%s", gitopsRepoDir, "terraform/github/repos.tf.tmpl")
	err = cp.Copy(tmplpath, path)

	if err != nil {
		log.Info().Msgf("Error problem copying %s to %s error: %s",
			tmplpath, path, err.Error())
		return err
	}

	pattern := fmt.Sprintf("s/GITOPS_REPO_NAME/\"%s\"/", gitopsRepoName)
	_, _, err = pkg.ExecShellReturnStrings("sed", "-i", pattern, path)

	if err != nil {
		log.Info().Msgf("Error problem replacing gitops directory name with gitopsRepoName=%s path=%s pattern=%s error: %s",
			gitopsRepoName, path, pattern, err.Error())
		return err
	}

	return err
}

func AdjustMetaphorRepo(destinationMetaphorRepoGitURL, gitopsRepoDir, metaphorRepoName, gitProvider, k1Dir string) error {

	//* create ~/.k1/metaphor
	metaphorDir := fmt.Sprintf("%s/metaphor", k1Dir)
	os.Mkdir(metaphorDir, 0700)

	//* git init
	metaphorRepo, err := git.PlainInit(metaphorDir, false)
	if err != nil {
		return err
	}

	//* copy options
	opt := cp.Options{
		Skip: func(src string) (bool, error) {
			if strings.HasSuffix(src, ".git") {
				return true, nil
			} else if strings.Index(src, "/.terraform") > 0 {
				return true, nil
			}
			//Add more stuff to be ignored here
			return false, nil

		},
	}

	//* metaphor app source
	metaphorContent := fmt.Sprintf("%s/metaphor", gitopsRepoDir)
	err = cp.Copy(metaphorContent, metaphorDir, opt)
	if err != nil {
		log.Info().Msgf("Error populating metaphor content with %s. error: %s", metaphorContent, err.Error())
		return err
	}

	//* copy ci content
	switch gitProvider {
	case "github":
		//* copy $HOME/.k1/gitops/ci/.github/* $HOME/.k1/metaphor/.github
		githubActionsFolderContent := fmt.Sprintf("%s/gitops/ci/.github", k1Dir)
		log.Info().Msgf("copying github content: %s", githubActionsFolderContent)
		err := cp.Copy(githubActionsFolderContent, fmt.Sprintf("%s/.github", metaphorDir), opt)
		if err != nil {
			log.Info().Msgf("error populating metaphor repository with %s: %s", githubActionsFolderContent, err)
			return err
		}
	case "gitlab":
		//* copy $HOME/.k1/gitops/ci/.gitlab-ci.yml/* $HOME/.k1/metaphor/.github
		gitlabCIContent := fmt.Sprintf("%s/gitops/ci/.gitlab-ci.yml", k1Dir)
		log.Info().Msgf("copying gitlab content: %s", gitlabCIContent)
		err := cp.Copy(gitlabCIContent, fmt.Sprintf("%s/.gitlab-ci.yml", metaphorDir), opt)
		if err != nil {
			log.Info().Msgf("error populating metaphor repository with %s: %s", gitlabCIContent, err)
			return err
		}
	}

	//* copy $HOME/.k1/gitops/ci/.argo/* $HOME/.k1/metaphor/.argo
	argoWorkflowsFolderContent := fmt.Sprintf("%s/gitops/ci/.argo", k1Dir)
	log.Info().Msgf("copying argo workflows content: %s", argoWorkflowsFolderContent)
	err = cp.Copy(argoWorkflowsFolderContent, fmt.Sprintf("%s/.argo", metaphorDir), opt)
	if err != nil {
		log.Info().Msgf("error populating metaphor repository with %s: %s", argoWorkflowsFolderContent, err)
		return err
	}

	//* copy $HOME/.k1/gitops/metaphor/Dockerfile $HOME/.k1/metaphor/build/Dockerfile
	dockerfileContent := fmt.Sprintf("%s/Dockerfile", metaphorDir)
	os.Mkdir(metaphorDir+"/build", 0700)
	log.Info().Msgf("copying dockerfile content: %s", argoWorkflowsFolderContent)
	err = cp.Copy(dockerfileContent, fmt.Sprintf("%s/build/Dockerfile", metaphorDir), opt)
	if err != nil {
		log.Info().Msgf("error populating metaphor repository with %s: %s", argoWorkflowsFolderContent, err)
		return err
	}
	os.RemoveAll(fmt.Sprintf("%s/ci", gitopsRepoDir))
	os.RemoveAll(fmt.Sprintf("%s/metaphor", gitopsRepoDir))

	//  add
	// commit
	err = gitClient.Commit(metaphorRepo, "committing initial detokenized metaphor repo content")
	if err != nil {
		return err
	}

	metaphorRepo, err = gitClient.SetRefToMainBranch(metaphorRepo)
	if err != nil {
		return err
	}

	// remove old git ref
	err = metaphorRepo.Storer.RemoveReference(plumbing.NewBranchReferenceName("master"))
	if err != nil {
		return fmt.Errorf("error removing previous git ref: %s", err)
	}

	// replace metaphore repo name in repos.tf
	path := fmt.Sprintf("%s/terraform/github/repos.tf", gitopsRepoDir)
	pattern := fmt.Sprintf("s/METAPHOR_REPO_NAME/\"%s\"/", metaphorRepoName)
	_, _, err = pkg.ExecShellReturnStrings("sed", "-i", pattern, path)
	if err != nil {
		return fmt.Errorf("error replacing gitops repo name in repos.tf: %s", err)
	}

	// create remote
	_, err = metaphorRepo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{destinationMetaphorRepoGitURL},
	})

	if err != nil {
		return fmt.Errorf("error problem creating Metaphore repo: URL=%s: %s",
			destinationMetaphorRepoGitURL, err)
	}

	return err
}
