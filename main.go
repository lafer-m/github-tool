package main

import (
	"context"
	"errors"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	ghttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v32/github"
	"github.com/prometheus/common/log"
	"github.com/spf13/cobra"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

func main() {
	cmd := newRootCmd(&Options{})
	if err := cmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

type Options struct {
	proxy     string
	org       string
	path      string
	username  string
	password  string
	gitClient *github.Client
}

func newRootCmd(options *Options) *cobra.Command {
	root := &cobra.Command{
		Use: "github",
	}
	root.PersistentFlags().StringVar(&options.proxy, "proxy", "", "http/https or socks5 proxy")
	root.AddCommand(newCloneCmd(options))
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		log.Infoln("proxy address ", options.proxy)
		return options.newGithubClient()
	}
	return root
}

func newCloneCmd(options *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone",
		Short: "clone github repo by org",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cloneReposByOrg(options)
		},
	}
	cmd.PersistentFlags().StringVar(&options.org, "org", "", "org name")
	cmd.PersistentFlags().StringVar(&options.path, "path", ".", "clone path")
	cmd.PersistentFlags().StringVar(&options.username, "username", "", "github username")
	cmd.PersistentFlags().StringVar(&options.password, "password", "", "github password")
	return cmd
}

// clone github repos by org
func cloneReposByOrg(options *Options) error {
	if options.org == "" {
		return errors.New("must have one org")
	}
	if options.gitClient == nil {
		return errors.New("not have client")
	}

	repos, err := options.listReposByOrg()
	if err != nil {
		return err
	}

	if err := options.setGitProxy(); err != nil {
		return err
	}

	for _, repo := range repos {
		log.Infoln("begin clone repo ", *repo.Name, *repo.URL)
		if _, err := git.PlainClone(filepath.Join(options.path, *repo.Name), false, &git.CloneOptions{
			URL:               *repo.CloneURL,
			RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
			Progress:          os.Stdout,
			Auth: &ghttp.BasicAuth{
				Username: options.username,
				Password: options.password,
			},
		}); err != nil {
			log.Errorln("clone repo err: ", err)
		}
	}

	log.Infoln(options)
	return nil
}

// install http proxy client for git library
func (options *Options) setGitProxy() error {
	httpClient, err := options.newHttpClient()
	if err != nil {
		return err
	}
	client.InstallProtocol("https", ghttp.NewClient(httpClient))
	client.InstallProtocol("http", ghttp.NewClient(httpClient))
	return nil
}

func (options *Options) newGithubClient() error {
	httpClient, err := options.newHttpClient()
	if err != nil {
		return err
	}
	options.gitClient = github.NewClient(httpClient)
	return nil
}

func (options *Options) newHttpClient() (*http.Client, error) {
	var httpClient *http.Client
	if options.proxy != "" {
		u, err := url.Parse(options.proxy)
		if err != nil {
			log.Error(err)
			return nil, err
		}
		tr := &http.Transport{
			Proxy: http.ProxyURL(u),
		}

		httpClient = &http.Client{
			Transport: tr,
		}
	}
	return httpClient, nil
}

// list repos by org
func (options *Options) listReposByOrg() ([]*github.Repository, error) {
	var allRepos []*github.Repository
	ops := &github.RepositoryListByOrgOptions{
		Type: "all",
		ListOptions: github.ListOptions{
			Page:    0,
			PerPage: 100,
		},
	}

	for {
		repos, resp, err := options.gitClient.Repositories.ListByOrg(context.Background(), options.org, ops)
		if err != nil {
			log.Error(err)
			return allRepos, err
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		ops.Page = resp.NextPage
	}
	return allRepos, nil
}
