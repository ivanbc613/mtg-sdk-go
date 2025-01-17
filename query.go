package mtg

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	queryUrl = "https://api.magicthegathering.io/v1/"
)

var (
	linkRE = regexp.MustCompile(`<(.*)>; rel="(.*)"`)
)

type CardColumn string

var (
	// CardName is the column for the name property.
	// For split, double-faced and flip cards, just the name of one side of the card. Basically each ‘sub-card’ has its own record.
	CardName = CardColumn("name")
	// CardLayout is the column for the layout property.
	// The card layout. Possible values: normal, split, flip, double-faced, token, plane, scheme, phenomenon, leveler, vanguard
	CardLayout = CardColumn("layout")
	// CardCMC is the column for the cmc property.
	// Converted mana cost. Always a number.
	CardCMC = CardColumn("cmc")
	// CardColors is the column for the colors property.
	// The card colors. Usually this is derived from the casting cost, but some cards are special (like the back of dual sided cards and Ghostfire).
	CardColors = CardColumn("colors")
	// CardColorIdentity is the column for the color identity property.
	// The card colors by color code. [“Red”, “Blue”] becomes [“R”, “U”]
	CardColorIdentity = CardColumn("colorIdentity")
	// CardType is the column for the type property.
	// The card type. This is the type you would see on the card if printed today. Note: The dash is a UTF8 'long dash’ as per the MTG rules
	CardType = CardColumn("type")
	// CardSupertypes is the column for the supertypes property.
	// The supertypes of the card. These appear to the far left of the card type. Example values: Basic, Legendary, Snow, World, Ongoing
	CardSupertypes = CardColumn("supertypes")
	// CardTypes is the column for the types property.
	// The types of the card. These appear to the left of the dash in a card type. Example values: Instant, Sorcery, Artifact, Creature, Enchantment, Land, Planeswalker
	CardTypes = CardColumn("types")
	// CardSubtypes is the column for the subtypes property.
	// The subtypes of the card. These appear to the right of the dash in a card type. Usually each word is its own subtype. Example values: Trap, Arcane, Equipment, Aura, Human, Rat, Squirrel, etc.
	CardSubtypes = CardColumn("subtypes")
	// CardRarity is the column for the rarity property.
	// The rarity of the card. Examples: Common, Uncommon, Rare, Mythic Rare, Special, Basic Land
	CardRarity = CardColumn("rarity")
	// CardSet is the column for the set property.
	// The set the card belongs to (set code).
	CardSet = CardColumn("set")
	// CardSetName is the column for the setName property.
	// The set the card belongs to.
	CardSetName = CardColumn("setName")
	// CardText is the column for the text property.
	// The oracle text of the card. May contain mana symbols and other symbols.
	CardText = CardColumn("text")
	// CardFlavor is the column for the flavor property.
	// The flavor text of the card.
	CardFlavor = CardColumn("flavor")
	// CardArtist is the column for the artist property.
	// The artist of the card. This may not match what is on the card as MTGJSON corrects many card misprints.
	CardArtist = CardColumn("artist")
	// CardNumber is the column for the number property.
	// The card number. This is printed at the bottom-center of the card in small text. This is a string, not an integer, because some cards have letters in their numbers.
	CardNumber = CardColumn("number")
	// CardPower is the column for the power property.
	// The power of the card. This is only present for creatures. This is a string, not an integer, because some cards have powers like: “1+*”
	CardPower = CardColumn("power")
	// CardToughness is the column for the toughness property.
	// The toughness of the card. This is only present for creatures. This is a string, not an integer, because some cards have toughness like: “1+*”
	CardToughness = CardColumn("toughness")
	// CardLoyalty is the column for the loyalty property.
	// The loyalty of the card. This is only present for planeswalkers.
	CardLoyalty = CardColumn("loyalty")
	// CardForeignName is the column for the foreign name property.
	// The name of a card in a foreign language it was printed in
	CardForeignName = CardColumn("foreignName")
	// CardLanguage is the column for the language property.
	// The language the card is printed in. Use this parameter when searching by foreignName
	CardLanguage = CardColumn("language")
	// CardGameFormat is the column for the game format property.
	// The game format, such as Commander, Standard, Legacy, etc. (when used, legality defaults to Legal unless supplied)
	CardGameFormat = CardColumn("gameFormat")
	// CardLegality is the column for the legality property.
	// The legality of the card for a given format, such as Legal, Banned or Restricted.
	CardLegality = CardColumn("legality")
)

// Query interface can be used to query multiple cards by their properties
type Query interface {
	// Where filters the given column by the given value
	Where(column CardColumn, qry string) Query
	// Sorts the query results by the given column
	OrderBy(column CardColumn) Query

	// Creates a copy of this query
	Copy() Query

	// Fetches all cards matching the current query
	All(debug ...bool) ([]*Card, error)

	// Fetches the given page of cards.
	Page(pageNum int, debug ...bool) (cards []*Card, totalCardCount int, err error)
	// Fetches one page of cards with a given page size
	PageS(pageNum int, pageSize int, debug ...bool) (cards []*Card, totalCardCount int, err error)
	// Fetches some random cards
	Random(count int, debug ...bool) ([]*Card, error)
}

// NewQuery creates a new Query to fetch cards
func NewQuery() Query {
	return make(query)
}

type query map[string]string

func fetchCards(url string, isDebug bool) ([]*Card, http.Header, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, err
	}

	if isDebug {
		fmt.Println("Request:")
		fmt.Println(url)
	}

	bdy := resp.Body
	defer bdy.Close()

	if err := checkError(resp); err != nil {
		return nil, nil, err
	}
	cards, err := decodeCards(bdy)
	if isDebug {
		fmt.Println("Decoded cards:")
		fmt.Printf("%+v\n", cards)
	}
	if err != nil {
		return nil, nil, err
	}
	return cards, resp.Header, nil
}

func (q query) All(debug ...bool) ([]*Card, error) {
	var allCards []*Card
	isDebug := false
	if len(debug) == 1 {
		isDebug = debug[0]
	}
	queryVals := make(url.Values)
	for k, v := range q {
		queryVals.Set(k, v)
	}
	nextUrl := queryUrl + "cards?" + queryVals.Encode()
	for nextUrl != "" {
		cards, header, err := fetchCards(nextUrl, isDebug)
		if err != nil {
			return nil, err
		}

		nextUrl = ""

		if linkH, ok := header["Link"]; ok {
			parts := strings.Split(linkH[0], ",")
			for _, link := range parts {
				match := linkRE.FindStringSubmatch(link)
				if match != nil {
					if match[2] == "next" {
						nextUrl = match[1]
					}
				}
			}
		}

		allCards = append(allCards, cards...)
	}
	return allCards, nil
}

func (q query) Page(pageNum int, debug ...bool) (cards []*Card, totalCardCount int, err error) {
	return q.PageS(pageNum, 100, debug...)
}

func (q query) PageS(pageNum int, pageSize int, debug ...bool) (cards []*Card, totalCardCount int, err error) {
	cards = nil
	totalCardCount = 0
	err = nil

	queryVals := make(url.Values)
	for k, v := range q {
		queryVals.Set(k, v)
	}

	isDebug := false
	if len(debug) == 1 {
		isDebug = debug[0]
	}

	queryVals.Set("page", strconv.Itoa(pageNum))
	queryVals.Set("pageSize", strconv.Itoa(pageSize))

	url := queryUrl + "cards?" + queryVals.Encode()
	cards, header, err := fetchCards(url, isDebug)
	if err != nil {
		return nil, 0, err
	}
	totalCardCount = len(cards)
	if totals, ok := header["Total-Count"]; ok && len(totals) > 0 {
		if totalCardCount, err = strconv.Atoi(totals[0]); err != nil {
			return nil, 0, err
		}
	}
	return cards, totalCardCount, nil
}

func (q query) Random(count int, debug ...bool) ([]*Card, error) {
	queryVals := make(url.Values)
	for k, v := range q {
		queryVals.Set(k, v)
	}

	isDebug := false
	if len(debug) == 1 {
		isDebug = debug[0]
	}

	queryVals.Set("random", "true")
	queryVals.Set("pageSize", strconv.Itoa(count))

	url := queryUrl + "cards?" + queryVals.Encode()
	cards, _, err := fetchCards(url, isDebug)
	return cards, err
}

func (q query) Copy() Query {
	r := make(query)
	for k, v := range q {
		r[k] = v
	}
	return r
}

func (q query) Where(column CardColumn, qry string) Query {
	q[string(column)] = qry
	return q
}

func (q query) OrderBy(column CardColumn) Query {
	q["orderBy"] = string(column)
	return q
}
