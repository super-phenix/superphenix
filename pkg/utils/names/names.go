package names

import "math/rand/v2"

var adjectives = []string{
	"agile", "bold", "brave", "bright", "calm", "clever", "cosmic", "crisp", "daring", "eager",
	"fancy", "fierce", "gentle", "glad", "grand", "happy", "humble", "jolly", "keen", "kind",
	"lively", "lucky", "merry", "mighty", "neat", "nimble", "noble", "patient", "proud", "quick",
	"quiet", "rapid", "sharp", "shiny", "silent", "smart", "smooth", "snappy", "steady", "sturdy",
	"sunny", "swift", "tidy", "vivid", "warm", "wild", "wise", "witty", "young", "zesty", "super",
}

var colors = []string{
	"amber", "apricot", "aqua", "azure", "beige", "black", "blue", "bronze", "brown", "cherry",
	"cobalt", "copper", "coral", "crimson", "cyan", "ebony", "emerald", "fuchsia", "gold", "green",
	"grey", "indigo", "ivory", "jade", "khaki", "lavender", "lemon", "lilac", "lime", "magenta",
	"maroon", "mauve", "mint", "navy", "ochre", "olive", "orange", "peach", "pink", "plum",
	"purple", "red", "rose", "ruby", "saffron", "scarlet", "silver", "teal", "violet", "white",
}

var animals = []string{
	"badger", "bear", "beaver", "bison", "camel", "cobra", "crane", "deer", "dolphin", "eagle",
	"falcon", "ferret", "finch", "fox", "gecko", "heron", "ibis", "jaguar", "koala", "lemur",
	"leopard", "lion", "lynx", "marmot", "mole", "moose", "newt", "otter", "owl", "panda",
	"panther", "parrot", "pelican", "penguin", "puffin", "puma", "rabbit", "raven", "robin", "salmon",
	"seal", "shark", "sparrow", "swan", "tiger", "toucan", "turtle", "walrus", "wolf", "zebra",
	"phenix", "finix",
}

// Generate returns a random name made of an adjective, a color and an animal
// joined by dashes (e.g. "brave-amber-otter").
func Generate() string {
	return pick(adjectives) + "-" + pick(colors) + "-" + pick(animals)
}

func pick(words []string) string {
	return words[rand.IntN(len(words))]
}
