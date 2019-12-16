package scraper

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/stashapp/stash/pkg/models"
)

type scraperAction string

const (
	scraperActionScript scraperAction = "script"
)

var allScraperAction = []scraperAction{
	scraperActionScript,
}

func (e scraperAction) IsValid() bool {
	switch e {
	case scraperActionScript:
		return true
	}
	return false
}

type scraperTypeConfig struct {
	Action scraperAction `yaml:"action"`
	Script []string      `yaml:"script,flow"`
}

type scrapePerformerNamesFunc func(c scraperTypeConfig, name string) ([]*models.ScrapedPerformer, error)

type performerByNameConfig struct {
	scraperTypeConfig `yaml:",inline"`
	performScrape     scrapePerformerNamesFunc
}

func (c *performerByNameConfig) resolveFn() {
	if c.Action == scraperActionScript {
		c.performScrape = scrapePerformerNamesScript
	}
}

type scrapePerformerFragmentFunc func(c scraperTypeConfig, scrapedPerformer models.ScrapedPerformerInput) (*models.ScrapedPerformer, error)

type performerByFragmentConfig struct {
	scraperTypeConfig `yaml:",inline"`
	performScrape     scrapePerformerFragmentFunc
}

func (c *performerByFragmentConfig) resolveFn() {
	if c.Action == scraperActionScript {
		c.performScrape = scrapePerformerFragmentScript
	}
}

type scrapeByURLConfig struct {
	scraperTypeConfig `yaml:",inline"`
	URL               []string `yaml:"url,flow"`
}

func (c scrapeByURLConfig) matchesURL(url string) bool {
	for _, thisURL := range c.URL {
		if strings.Contains(url, thisURL) {
			return true
		}
	}

	return false
}

type scrapePerformerByURLFunc func(c scraperTypeConfig, url string) (*models.ScrapedPerformer, error)

type scrapePerformerByURLConfig struct {
	scrapeByURLConfig `yaml:",inline"`
	performScrape     scrapePerformerByURLFunc
}

func (c *scrapePerformerByURLConfig) resolveFn() {
	if c.Action == scraperActionScript {
		c.performScrape = scrapePerformerURLScript
	}
}

type scrapeSceneFragmentFunc func(c scraperTypeConfig, scene models.SceneUpdateInput) (*models.ScrapedScene, error)

type sceneByFragmentConfig struct {
	scraperTypeConfig `yaml:",inline"`
	performScrape     scrapeSceneFragmentFunc
}

func (c *sceneByFragmentConfig) resolveFn() {
	if c.Action == scraperActionScript {
		c.performScrape = scrapeSceneFragmentScript
	}
}

type scrapeSceneByURLFunc func(c scraperTypeConfig, url string) (*models.ScrapedScene, error)

type scrapeSceneByURLConfig struct {
	scrapeByURLConfig `yaml:",inline"`
	performScrape     scrapeSceneByURLFunc
}

func (c *scrapeSceneByURLConfig) resolveFn() {
	if c.Action == scraperActionScript {
		c.performScrape = scrapeSceneURLScript
	}
}

type scraperConfig struct {
	ID                  string
	Name                string                        `yaml:"name"`
	PerformerByName     *performerByNameConfig        `yaml:"performerByName"`
	PerformerByFragment *performerByFragmentConfig    `yaml:"performerByFragment"`
	PerformerByURL      []*scrapePerformerByURLConfig `yaml:"performerByURL"`
	SceneByFragment     *sceneByFragmentConfig        `yaml:"sceneByFragment"`
	SceneByURL          []*scrapeSceneByURLConfig     `yaml:"sceneByURL"`
}

func loadScraperFromYAML(path string) (*scraperConfig, error) {
	ret := &scraperConfig{}

	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return nil, err
	}
	parser := yaml.NewDecoder(file)
	parser.SetStrict(true)
	err = parser.Decode(&ret)
	if err != nil {
		return nil, err
	}

	// set id to the filename
	id := filepath.Base(path)
	id = id[:strings.LastIndex(id, ".")]
	ret.ID = id

	// set the scraper interface
	ret.initialiseConfigs()

	return ret, nil
}

func (c *scraperConfig) initialiseConfigs() {
	if c.PerformerByName != nil {
		c.PerformerByName.resolveFn()
	}
	if c.PerformerByFragment != nil {
		c.PerformerByFragment.resolveFn()
	}
	for _, s := range c.PerformerByURL {
		s.resolveFn()
	}

	if c.SceneByFragment != nil {
		c.SceneByFragment.resolveFn()
	}
	for _, s := range c.SceneByURL {
		s.resolveFn()
	}
}

func (c scraperConfig) toScraper() *models.Scraper {
	ret := models.Scraper{
		ID:   c.ID,
		Name: c.Name,
	}

	performer := models.ScraperSpec{}
	if c.PerformerByName != nil {
		performer.SupportedScrapes = append(performer.SupportedScrapes, models.ScrapeTypeName)
	}
	if c.PerformerByFragment != nil {
		performer.SupportedScrapes = append(performer.SupportedScrapes, models.ScrapeTypeFragment)
	}
	if len(c.PerformerByURL) > 0 {
		performer.SupportedScrapes = append(performer.SupportedScrapes, models.ScrapeTypeURL)
		for _, v := range c.PerformerByURL {
			performer.Urls = append(performer.Urls, v.URL...)
		}
	}

	if len(performer.SupportedScrapes) > 0 {
		ret.Performer = &performer
	}

	scene := models.ScraperSpec{}
	if c.SceneByFragment != nil {
		scene.SupportedScrapes = append(scene.SupportedScrapes, models.ScrapeTypeFragment)
	}
	if len(c.SceneByURL) > 0 {
		scene.SupportedScrapes = append(scene.SupportedScrapes, models.ScrapeTypeURL)
		for _, v := range c.SceneByURL {
			scene.Urls = append(scene.Urls, v.URL...)
		}
	}

	if len(scene.SupportedScrapes) > 0 {
		ret.Scene = &scene
	}

	return &ret
}

func (c scraperConfig) supportsPerformers() bool {
	return c.PerformerByName != nil || c.PerformerByFragment != nil || len(c.PerformerByURL) > 0
}

func (c scraperConfig) matchesPerformerURL(url string) bool {
	for _, scraper := range c.PerformerByURL {
		if scraper.matchesURL(url) {
			return true
		}
	}

	return false
}

func (c scraperConfig) ScrapePerformerNames(name string) ([]*models.ScrapedPerformer, error) {
	if c.PerformerByName != nil && c.PerformerByName.performScrape != nil {
		return c.PerformerByName.performScrape(c.PerformerByName.scraperTypeConfig, name)
	}

	return nil, nil
}

func (c scraperConfig) ScrapePerformer(scrapedPerformer models.ScrapedPerformerInput) (*models.ScrapedPerformer, error) {
	if c.PerformerByFragment != nil && c.PerformerByFragment.performScrape != nil {
		return c.PerformerByFragment.performScrape(c.PerformerByFragment.scraperTypeConfig, scrapedPerformer)
	}

	return nil, nil
}

func (c scraperConfig) ScrapePerformerURL(url string) (*models.ScrapedPerformer, error) {
	for _, scraper := range c.PerformerByURL {
		if scraper.matchesURL(url) && scraper.performScrape != nil {
			ret, err := scraper.performScrape(scraper.scraperTypeConfig, url)
			if err != nil {
				return nil, err
			}

			if ret != nil {
				return ret, nil
			}
		}
	}

	return nil, nil
}

func (c scraperConfig) supportsScenes() bool {
	return c.SceneByFragment != nil || len(c.SceneByURL) > 0
}

func (c scraperConfig) matchesSceneURL(url string) bool {
	for _, scraper := range c.SceneByURL {
		if scraper.matchesURL(url) {
			return true
		}
	}

	return false
}

func (c scraperConfig) ScrapeScene(scene models.SceneUpdateInput) (*models.ScrapedScene, error) {
	if c.SceneByFragment != nil && c.SceneByFragment.performScrape != nil {
		return c.SceneByFragment.performScrape(c.SceneByFragment.scraperTypeConfig, scene)
	}

	return nil, nil
}

func (c scraperConfig) ScrapeSceneURL(url string) (*models.ScrapedScene, error) {
	for _, scraper := range c.SceneByURL {
		if scraper.matchesURL(url) && scraper.performScrape != nil {
			ret, err := scraper.performScrape(scraper.scraperTypeConfig, url)
			if err != nil {
				return nil, err
			}

			if ret != nil {
				return ret, nil
			}
		}
	}

	return nil, nil
}
