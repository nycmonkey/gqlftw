package graph

import (
	"bytes"
	context "context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/knakk/sparql"

	model "github.com/nycmonkey/gqlftw/model"
)

// MyApp serves GraphQL
type MyApp struct {
	// initial list of companies is loaded once to limit hits to the backend service
	companies []model.Company
	cIdx      map[string]int // maps URI to index in companies
	qbank     sparql.Bank
	repo      *sparql.Repo
}

// New returns a GraphQL resolver
func New() (app *MyApp, err error) {
	a := MyApp{
		qbank: sparql.LoadBank(bytes.NewBufferString(queries)),
		cIdx:  make(map[string]int),
	}
	a.repo, err = sparql.NewRepo("https://query.wikidata.org/sparql",
		sparql.Timeout(time.Second*2))
	if err != nil {
		return
	}
	var q string
	q, err = a.qbank.Prepare(`load-company-query`)
	if err != nil {
		return
	}
	log.Println(q)
	reply, err := a.repo.Query(q)
	if err != nil {
		return
	}
	people := make(map[string]map[string]struct{}) // keys: company uri, employee uri
	for _, tMap := range reply.Solutions() {
		ppl, ok := people[tMap[`corp`].String()]
		if ok {
			ppl[tMap[`emp`].String()] = struct{}{}
			people[tMap[`corp`].String()] = ppl
			continue
		}
		c := model.Company{
			WikidataURI:  tMap[`corp`].String(),
			Name:         tMap[`corpLabel`].String(),
			Homepage:     tMap[`homepage`].String(),
			ExecutiveURI: tMap[`exec`].String(),
			LogoURL:      tMap[`logoLabel`].String(),
		}
		a.cIdx[c.WikidataURI] = len(a.companies)
		a.companies = append(a.companies, c)
		people[c.WikidataURI] = make(map[string]struct{})
		people[c.WikidataURI][tMap[`emp`].String()] = struct{}{}

	}
	for companyURI, peopleURIs := range people {
		a.companies[a.cIdx[companyURI]].PeopleURIs = make([]string, 0, len(peopleURIs))
		for uri := range peopleURIs {
			a.companies[a.cIdx[companyURI]].PeopleURIs = append(a.companies[a.cIdx[companyURI]].PeopleURIs, uri)
		}
	}
	return &a, err
}

// Company_executive resolves the head of the company according to Wikidata
func (a *MyApp) Company_executive(ctx context.Context, obj *model.Company) (*model.Person, error) {
	return a.loadPerson(obj.ExecutiveURI, obj.WikidataURI)
}

// Company_employees resolves the employees of a company known to Wikidata
func (a *MyApp) Company_employees(ctx context.Context, obj *model.Company) (results []model.Person, err error) {
	// this really sucks without a dataloader or batch call
	results = make([]model.Person, 0, len(obj.PeopleURIs))
	for _, personURI := range obj.PeopleURIs {
		var p *model.Person
		p, err = a.loadPerson(personURI, obj.WikidataURI)
		if err != nil {
			return
		}
		if p != nil {
			results = append(results, *p)
		}
	}
	return
}

// Person_employer resolves the person's employer
func (a *MyApp) Person_employer(ctx context.Context, obj *model.Person) (*model.Company, error) {
	idx, ok := a.cIdx[obj.EmployerURI]
	if !ok {
		return nil, fmt.Errorf(`company index mapped incorrectly: employer URI '%s' not found`, obj.EmployerURI)
	}
	if idx >= len(a.companies) {
		return nil, fmt.Errorf(`company index mapped incorrectly: indexed URI out of range`)
	}
	return &a.companies[idx], nil
}

// Query_company returns the first company whose website contains domain
func (a *MyApp) Query_company(ctx context.Context, domain string) (*model.Company, error) {
	q := domain
	if strings.Contains(domain, `://`) {
		q = strings.ToLower(domain[strings.Index(domain, `://`):])
	}
	q = strings.TrimSuffix(q, `/`)
	for _, c := range a.companies {
		if strings.Contains(strings.ToLower(c.Homepage), domain) {
			return &c, nil
		}
	}
	return nil, fmt.Errorf(`no companies found with domain '%s'`, domain)
}

func (a *MyApp) Query_companies(ctx context.Context) ([]model.Company, error) {
	return a.companies, nil
}

func (a *MyApp) loadPerson(personURI, companyURI string) (*model.Person, error) {
	q, err := a.qbank.Prepare(`load-person-query`, struct{ URIs []string }{URIs: []string{personURI}})
	if err != nil {
		return nil, err
	}
	log.Println(q)
	result, err := a.repo.Query(q)
	if err != nil {
		return nil, err
	}
	sols := result.Solutions()
	if len(sols) == 0 {
		return nil, fmt.Errorf(`no person found with URI '%s'`, personURI)
	}
	p := model.Person{
		EmployerURI: companyURI,
		WikidataURI: personURI,
		Name:        sols[0][`personLabel`].String(),
		PhotoURLs:   make([]string, 0, len(sols)),
	}
	for _, sol := range sols {
		p.PhotoURLs = append(p.PhotoURLs, sol[`personPhotoLabel`].String())
	}
	return &p, nil
}

const queries = `
# tag: load-company-query
PREFIX wdt: <http://www.wikidata.org/prop/direct/>
PREFIX wikibase: <http://wikiba.se/ontology#>
SELECT ?corp ?corpLabel ?homepage ?exec ?logoLabel ?emp WHERE {
  ?corp wdt:P169|wdt:P2828|wdt:P1037 ?exec .
  ?corp wdt:P31 wd:Q4830453 .
  ?corp wdt:P856 ?homepage .
  ?corp wdt:P154 ?logo .
  OPTIONAL { ?emp wdt:P108 ?corp ;
	wdt:P18 ?empPhoto .
  }
  FILTER ( ?emp != ?exec )
  SERVICE wikibase:label { bd:serviceParam wikibase:language "en". }
}

# tag: load-person-query
PREFIX wdt: <http://www.wikidata.org/prop/direct/>
PREFIX wikibase: <http://wikiba.se/ontology#>
SELECT ?person ?personLabel ?personPhotoLabel WHERE {
  OPTIONAL { ?person wdt:P18 ?personPhoto . }
  VALUES ( ?person ) { {{ range .URIs }} (<{{.}}>) {{ end }} }
  
  SERVICE wikibase:label { bd:serviceParam wikibase:language "en". }
}
`
