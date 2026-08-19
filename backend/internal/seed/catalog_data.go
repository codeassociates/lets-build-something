package seed

// The fictional yard this system runs. Rates are in cents and roughly track
// what a regional hire shop charges, so the numbers on screen look plausible
// rather than obviously invented.

type categorySeed struct {
	Slug, Name, Description string
	Sort                    int
}

var categories = []categorySeed{
	{"breaking", "Demolition & Breaking", "Breakers, jackhammers and demolition hammers for concrete, asphalt and masonry.", 1},
	{"concrete", "Concrete & Masonry", "Mixers, screeds, trowels and saws for pours, finishing and cutting.", 2},
	{"pumps", "Pumps & Water", "Submersible, trash and high-head pumps for dewatering and transfer.", 3},
	{"painting", "Painting & Coatings", "Airless sprayers, texture rigs and surface preparation gear.", 4},
	{"compaction", "Compaction & Earth", "Plate compactors, rammers and rollers for base and backfill.", 5},
	{"power", "Generators & Air", "Portable generators, compressors and air tooling.", 6},
	{"access", "Lifting & Access", "Scissor lifts, material hoists and engine cranes.", 7},
	{"climate", "Heating & Drying", "Heaters, dehumidifiers and air movers for drying and winter work.", 8},
}

type modelSeed struct {
	Category                             string
	SKU, Name, Manufacturer, Description string
	Daily, Weekly, Monthly, Deposit      int64
	Replacement                          int64
	RequiresLicense                      bool
	Units                                int
	Specs                                map[string]string
}

var models = []modelSeed{
	// --- breaking ---
	{"breaking", "BRK-1400", "14 lb Electric Breaker", "Bosch",
		"Corded demolition hammer for slab work and light foundation breaking. Comes with point and flat chisels.",
		6500, 26000, 78000, 20000, 185000, false, 8,
		map[string]string{"Weight": "14 lb", "Power": "120V / 11A", "Impact energy": "10.8 ft-lb", "Chuck": "SDS-Max"}},
	{"breaking", "BRK-6000", "60 lb Pneumatic Jackhammer", "Chicago Pneumatic",
		"Heavy pavement breaker for road and thick slab demolition. Requires a 185 CFM compressor.",
		9500, 38000, 114000, 35000, 320000, false, 6,
		map[string]string{"Weight": "60 lb", "Air required": "90 CFM", "Blows/min": "1,100", "Shank": "1-1/8\" hex"}},
	{"breaking", "BRK-2800", "28 lb Demolition Hammer", "Hilti",
		"Mid-weight breaker that handles wall openings and footing removal without a compressor.",
		7800, 31000, 93000, 25000, 245000, false, 5,
		map[string]string{"Weight": "28 lb", "Power": "120V / 15A", "Impact energy": "22 ft-lb", "Chuck": "SDS-Max"}},
	{"breaking", "BRK-HYD1", "Hydraulic Breaker Attachment", "Okada",
		"Skid-steer mounted breaker for large demolition. Operator must hold a current plant ticket.",
		34000, 136000, 408000, 100000, 1450000, true, 2,
		map[string]string{"Carrier": "Skid steer, 60-90 hp", "Impact energy": "750 ft-lb", "Flow": "16-24 GPM"}},

	// --- concrete ---
	{"concrete", "CON-MIX9", "9 cu ft Concrete Mixer", "Multiquip",
		"Towable drum mixer with a honda petrol engine, sized for footings and small pours.",
		8500, 34000, 102000, 25000, 380000, false, 4,
		map[string]string{"Drum capacity": "9 cu ft", "Engine": "Honda GX160", "Tow": "Ball hitch, 2\""}},
	{"concrete", "CON-SAW14", "14\" Cut-Off Saw", "Husqvarna",
		"Petrol handheld saw for concrete, block and rebar. Diamond blade included; consumable is charged on wear.",
		7200, 28800, 86400, 20000, 145000, false, 7,
		map[string]string{"Blade": "14 in diamond", "Cut depth": "5 in", "Engine": "73 cc two-stroke", "Weight": "22 lb"}},
	{"concrete", "CON-TRW46", "46\" Ride-On Power Trowel", "Allen Engineering",
		"Ride-on trowel for finishing slabs over 5,000 sq ft. Delivery recommended.",
		29000, 116000, 348000, 75000, 1850000, true, 2,
		map[string]string{"Path width": "46 in", "Engine": "Honda GX390", "Blades": "8", "Weight": "620 lb"}},
	{"concrete", "CON-SCR12", "12 ft Vibrating Screed", "Wacker Neuson",
		"Wet screed for levelling slabs and driveways ahead of float finishing.",
		9800, 39000, 117000, 25000, 265000, false, 3,
		map[string]string{"Blade": "12 ft magnesium", "Engine": "Honda GX35", "Weight": "78 lb"}},
	{"concrete", "CON-VIB2", "Concrete Vibrator, 2\" Head", "Wyco",
		"Flexible-shaft immersion vibrator for consolidating pours around rebar.",
		4800, 19200, 57600, 10000, 78000, false, 6,
		map[string]string{"Head": "2 in", "Shaft": "14 ft flexible", "Power": "120V"}},

	// --- pumps ---
	{"pumps", "PMP-TR3", "3\" Trash Pump", "Honda",
		"Handles water with solids up to 1-1/8 in. The workhorse for flooded excavations.",
		7500, 30000, 90000, 20000, 165000, false, 6,
		map[string]string{"Ports": "3 in", "Flow": "290 GPM", "Max head": "92 ft", "Engine": "Honda GX240"}},
	{"pumps", "PMP-SUB2", "2\" Submersible Pump", "Tsurumi",
		"Electric submersible for continuous dewatering of pits and basements.",
		5200, 20800, 62400, 15000, 98000, false, 8,
		map[string]string{"Ports": "2 in", "Flow": "132 GPM", "Max head": "56 ft", "Power": "120V / 9.5A"}},
	{"pumps", "PMP-HH4", "4\" High-Head Diesel Pump", "Godwin",
		"Trailer-mounted pump for long runs and high lift. Diesel supplied full; returned full.",
		38000, 152000, 456000, 90000, 2200000, false, 2,
		map[string]string{"Ports": "4 in", "Flow": "600 GPM", "Max head": "230 ft", "Fuel": "Diesel, 30 gal"}},
	{"pumps", "PMP-HOSE", "20 ft Discharge Hose Set", "Generic",
		"Layflat discharge hose with camlock fittings. Rented alongside any pump.",
		1500, 6000, 18000, 2500, 22000, false, 14,
		map[string]string{"Length": "20 ft", "Diameter": "3 in", "Fittings": "Camlock"}},

	// --- painting ---
	{"painting", "PNT-AL55", "Airless Sprayer, 0.55 GPM", "Graco",
		"Contractor-grade airless for interior and exterior work up to 300 gallons a year.",
		9500, 38000, 114000, 25000, 210000, false, 5,
		map[string]string{"Flow": "0.55 GPM", "Max tip": "0.021 in", "Hose": "50 ft", "Power": "120V"}},
	{"painting", "PNT-TX40", "Texture & Drywall Sprayer", "Graco",
		"Hopper-fed texture rig for knockdown, orange peel and popcorn ceilings.",
		13500, 54000, 162000, 35000, 385000, false, 3,
		map[string]string{"Hopper": "12 gal", "Compressor": "Integrated", "Hose": "50 ft"}},
	{"painting", "PNT-SB80", "Portable Sandblaster, 80 lb", "Clemco",
		"Pressure pot blaster for rust and coating removal. Media and hood not included.",
		11000, 44000, 132000, 30000, 175000, false, 3,
		map[string]string{"Pot capacity": "80 lb", "Air required": "50 CFM", "Nozzle": "3/16 in"}},
	{"painting", "PNT-HEAT", "Infrared Paint Remover", "Speedheater",
		"Low-temperature infrared stripper for lead-safe paint removal on trim and siding.",
		5800, 23200, 69600, 15000, 68000, false, 4,
		map[string]string{"Power": "120V / 9A", "Coverage": "1 sq ft per pass", "Temp": "380-580°F"}},

	// --- compaction ---
	{"compaction", "CMP-PL20", "20\" Plate Compactor", "Wacker Neuson",
		"Forward plate for granular base under slabs, pavers and footings.",
		7800, 31200, 93600, 20000, 195000, false, 6,
		map[string]string{"Plate width": "20 in", "Force": "3,800 lbf", "Engine": "Honda GX160", "Weight": "185 lb"}},
	{"compaction", "CMP-RAM", "Jumping Jack Rammer", "Wacker Neuson",
		"Trench rammer for cohesive soils and narrow backfill where a plate cannot reach.",
		8200, 32800, 98400, 22000, 225000, false, 5,
		map[string]string{"Shoe": "11 in", "Force": "3,350 lbf", "Engine": "Honda GXR120", "Weight": "150 lb"}},
	{"compaction", "CMP-RLR3", "3 ft Walk-Behind Roller", "Multiquip",
		"Double-drum vibratory roller for asphalt patching and driveway work.",
		21000, 84000, 252000, 55000, 980000, false, 2,
		map[string]string{"Drum width": "35 in", "Force": "7,200 lbf", "Engine": "Diesel, 13 hp", "Weight": "3,100 lb"}},

	// --- power ---
	{"power", "PWR-GEN7", "7000W Portable Generator", "Honda",
		"Quiet inverter generator for site power, with 30A twist-lock and standard outlets.",
		9500, 38000, 114000, 25000, 480000, false, 6,
		map[string]string{"Output": "7,000W surge / 5,500W run", "Outlets": "30A twist-lock, 4× 120V", "Runtime": "10 h at 50%"}},
	{"power", "PWR-GEN20", "20 kW Towable Generator", "Multiquip",
		"Trailer-mounted diesel generator for whole-site power or events.",
		42000, 168000, 504000, 120000, 3800000, false, 2,
		map[string]string{"Output": "20 kW", "Voltage": "120/240V single, 208V three", "Fuel": "Diesel, 45 gal"}},
	{"power", "PWR-CMP185", "185 CFM Towable Compressor", "Atlas Copco",
		"Diesel compressor sized to run two 60 lb breakers. Hose and oiler included.",
		27000, 108000, 324000, 75000, 2650000, false, 3,
		map[string]string{"Flow": "185 CFM", "Pressure": "100 psi", "Fuel": "Diesel, 25 gal", "Hose": "50 ft, 3/4 in"}},
	{"power", "PWR-CMP2", "2 hp Electric Compressor", "DeWalt",
		"Quiet oil-free compressor for finish nailing and light air tooling indoors.",
		4200, 16800, 50400, 10000, 62000, false, 7,
		map[string]string{"Tank": "15 gal", "Flow": "5.0 CFM at 90 psi", "Power": "120V / 15A"}},

	// --- access ---
	{"access", "ACC-SL19", "19 ft Scissor Lift", "Genie",
		"Electric slab scissor lift for interior fit-out. Operator must hold a current lift certification.",
		24000, 96000, 288000, 70000, 1750000, true, 4,
		map[string]string{"Platform height": "19 ft", "Capacity": "500 lb", "Width": "32 in", "Power": "Electric, 24V"}},
	{"access", "ACC-HST1", "Material Hoist, 1 Ton", "Genie",
		"Manual material lift for HVAC units, beams and sheet goods.",
		8800, 35200, 105600, 22000, 285000, false, 4,
		map[string]string{"Capacity": "1,000 lb", "Max height": "12 ft", "Weight": "310 lb"}},
	{"access", "ACC-CRN2", "2 Ton Engine Crane", "Sunex",
		"Folding shop crane for engine and machinery lifts.",
		5500, 22000, 66000, 15000, 88000, false, 3,
		map[string]string{"Capacity": "4,000 lb", "Max lift": "7 ft", "Boom": "4 position"}},

	// --- climate ---
	{"climate", "CLM-HT400", "400k BTU Indirect Heater", "Frost Fighter",
		"Indirect-fired diesel heater delivering dry, fume-free heat for occupied spaces.",
		18500, 74000, 222000, 45000, 985000, false, 3,
		map[string]string{"Output": "400,000 BTU", "Fuel": "Diesel / kerosene", "Ducting": "2 × 12 in outlets"}},
	{"climate", "CLM-DH150", "150 Pint Dehumidifier", "Dri-Eaz",
		"LGR dehumidifier for water damage restoration and curing control.",
		7500, 30000, 90000, 20000, 245000, false, 6,
		map[string]string{"Extraction": "150 pints/day", "Airflow": "325 CFM", "Power": "120V / 8.9A"}},
	{"climate", "CLM-AM3", "Axial Air Mover", "Dri-Eaz",
		"Stackable air mover for drying floors and walls. Usually rented in threes.",
		2200, 8800, 26400, 5000, 42000, false, 18,
		map[string]string{"Airflow": "3,000 CFM", "Power": "120V / 2.5A", "Stackable": "Yes, 4 high"}},
	{"climate", "CLM-HT80", "80k BTU Forced Air Heater", "Mr. Heater",
		"Direct-fired torpedo heater for open sites and unoccupied structures.",
		4800, 19200, 57600, 12000, 58000, false, 8,
		map[string]string{"Output": "80,000 BTU", "Fuel": "Kerosene / diesel", "Tank": "10 gal"}},
}
