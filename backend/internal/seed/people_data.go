package seed

// Synthetic customers. A believable mix of trades, contractors and homeowners,
// because the customer list is one of the first things anyone looks at.

type personSeed struct {
	Name, Email, Phone, Company  string
	Address, City, State, Postal string
	License                      string
}

var staffMembers = []personSeed{
	{"Marisol Vega", "marisol@kestrelrental.example", "(406) 555-0111", "Kestrel Equipment Rental",
		"1420 Gallatin Road", "Bozeman", "MT", "59718", ""},
	{"Terrence Boyd", "terrence@kestrelrental.example", "(406) 555-0112", "Kestrel Equipment Rental",
		"1420 Gallatin Road", "Bozeman", "MT", "59718", ""},
	{"Priya Raghunathan", "priya@kestrelrental.example", "(406) 555-0113", "Kestrel Equipment Rental",
		"1420 Gallatin Road", "Bozeman", "MT", "59718", ""},
}

var customers = []personSeed{
	{"Dana Whitfield", "dana.whitfield@example.com", "(406) 555-0201", "Whitfield Concrete LLC",
		"88 Bridger Canyon Road", "Bozeman", "MT", "59715", "MT-D4419827"},
	{"Aaron Kowalczyk", "aaron.k@example.com", "(406) 555-0202", "Kowalczyk Excavation",
		"2210 Frontage Road", "Belgrade", "MT", "59714", "MT-D2287341"},
	{"Nadia Osei", "nadia.osei@example.com", "(406) 555-0203", "Osei Restoration Services",
		"14 Peach Street", "Bozeman", "MT", "59715", "MT-D8871209"},
	{"Grant Delacroix", "grant.d@example.com", "(406) 555-0204", "",
		"471 Sourdough Road", "Bozeman", "MT", "59715", ""},
	{"Ingrid Salvatierra", "ingrid.s@example.com", "(406) 555-0205", "Salvatierra Painting Co",
		"903 North 7th Avenue", "Bozeman", "MT", "59715", "MT-D3390118"},
	{"Bo Nakamura", "bo.nakamura@example.com", "(406) 555-0206", "Nakamura Framing",
		"55 Story Mill Road", "Bozeman", "MT", "59715", ""},
	{"Rosalind Achterberg", "roz.a@example.com", "(406) 555-0207", "Achterberg Property Group",
		"1200 Baxter Lane", "Bozeman", "MT", "59718", "MT-D5512778"},
	{"Emeka Nwachukwu", "emeka.n@example.com", "(406) 555-0208", "Nwachukwu Mechanical",
		"340 Griffin Drive", "Bozeman", "MT", "59715", ""},
	{"Sunniva Lindqvist", "sunniva.l@example.com", "(406) 555-0209", "",
		"77 Kagy Boulevard", "Bozeman", "MT", "59715", ""},
	{"Marcus Threadgill", "marcus.t@example.com", "(406) 555-0210", "Threadgill & Sons Roofing",
		"612 East Main Street", "Bozeman", "MT", "59715", "MT-D9903442"},
	{"Yuki Brannigan", "yuki.b@example.com", "(406) 555-0211", "Brannigan Interiors",
		"18 South Willson Avenue", "Bozeman", "MT", "59715", ""},
	{"Desmond Achebe", "des.achebe@example.com", "(406) 555-0212", "Achebe Site Works",
		"2905 Jackrabbit Lane", "Belgrade", "MT", "59714", "MT-D1147600"},
	{"Colleen Fitzwilliam", "colleen.f@example.com", "(406) 555-0213", "",
		"440 West Babcock Street", "Bozeman", "MT", "59715", ""},
	{"Rafael Ostrowski", "rafael.o@example.com", "(406) 555-0214", "Ostrowski Hardscapes",
		"1875 Durston Road", "Bozeman", "MT", "59718", "MT-D7784013"},
	{"Anneke Vermeulen", "anneke.v@example.com", "(406) 555-0215", "Vermeulen Custom Homes",
		"66 Cottonwood Road", "Bozeman", "MT", "59718", "MT-D2205567"},
	{"Tobias Amadi", "tobias.a@example.com", "(406) 555-0216", "",
		"219 North Rouse Avenue", "Bozeman", "MT", "59715", ""},
	{"Priscilla Hartmann", "priscilla.h@example.com", "(406) 555-0217", "Hartmann Flood & Fire",
		"1533 Oak Street", "Bozeman", "MT", "59715", "MT-D6620394"},
	{"Iker Bengoetxea", "iker.b@example.com", "(406) 555-0218", "Bengoetxea Masonry",
		"780 Bozeman Trail Road", "Bozeman", "MT", "59715", ""},
	{"Marguerite Osei-Tutu", "marguerite.ot@example.com", "(406) 555-0219", "Cardinal Facilities",
		"4120 Valley Commons Drive", "Bozeman", "MT", "59718", "MT-D4408821"},
	{"Sven Halvorsen", "sven.h@example.com", "(406) 555-0220", "Halvorsen Plumbing",
		"92 Wheat Drive", "Bozeman", "MT", "59718", ""},
	{"Leilani Kahananui", "leilani.k@example.com", "(406) 555-0221", "",
		"305 South Third Avenue", "Bozeman", "MT", "59715", ""},
	{"Bartholomew Quigley", "bart.q@example.com", "(406) 555-0222", "Quigley Demolition",
		"1601 Wagon Wheel Road", "Belgrade", "MT", "59714", "MT-D3378025"},
	{"Farida Benali", "farida.b@example.com", "(406) 555-0223", "Benali Design Build",
		"27 Lindley Place", "Bozeman", "MT", "59715", "MT-D5591230"},
	{"Callum Strachan", "callum.s@example.com", "(406) 555-0224", "",
		"850 Highland Boulevard", "Bozeman", "MT", "59715", ""},
}

// Job notes attached to reservations, so the desk view reads like real traffic.
var jobNotes = []string{
	"Basement slab removal — customer bringing own trailer.",
	"Driveway replacement on Sourdough. Needs delivery before 7am.",
	"Water damage callout, second floor. Rush.",
	"Retaining wall footings. Extending if weather turns.",
	"Interior repaint, 4,200 sq ft. Sprayer plus scaffold.",
	"Crawl space dewatering after the thaw.",
	"Shop floor grind and seal.",
	"Garage extension footings.",
	"Storm cleanup — culvert washout on Bridger Canyon.",
	"Tenant improvement, ceiling texture and paint.",
	"Winter pour, needs heat and hoarding.",
	"Fence post holes then compaction on the drive.",
	"",
	"",
	"Customer requested the same unit as last time if free.",
	"Account customer — invoice to office, do not charge card.",
	"Delivery to site, not collecting from the yard.",
	"Weekend job, returning Monday first thing.",
}
