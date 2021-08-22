package cli

import (
	"strings"
)

type Register interface {
	RegisterFlag(flag Flag) error
	RegisterArg(arg Arg) error
}

type Parser interface {
	Register
	Parse(arguments []string) error
	Args() []Arg
	Flags() []Flag
}

type flags struct {
	data  []Flag
	long  map[string]int // Indexes of flags in the data. It contains all long names.
	short map[string]int // Indexes of flags in the data. It contains all short names.
}

func (f *flags) Get(name string) (idx int, flag *Flag, ok bool) {
	idx, ok = f.long[name]
	if !ok {
		idx, ok = f.short[name]
	}

	if ok {
		flag = &f.data[idx]
	}

	return
}

func (f *flags) Find(long, short string, aliases []Alias) (idx int, flag *Flag, ok bool) {
	if long != "" {
		if idx, ok = f.long[long]; ok {
			flag = &f.data[idx]
			return
		}
	}

	if short != "" {
		if idx, ok = f.short[short]; ok {
			flag = &f.data[idx]
			return
		}
	}

	for _, alias := range aliases {
		if alias.Long != "" {
			if idx, ok = f.long[alias.Long]; ok {
				flag = &f.data[idx]
				return
			}
		}

		if alias.Short != "" {
			if idx, ok = f.short[alias.Short]; ok {
				flag = &f.data[idx]
				return
			}
		}
	}

	return
}

func (f *flags) Add(flag Flag) {
	// Find already added flag.
	var (
		idx   int
		found bool
	)
	if flag.Long != "" {
		idx, found = f.long[flag.Long]
		if found {
			f.data[idx] = flag
		}
	}

	if !found {
		if flag.Short != "" {
			idx, found = f.short[flag.Short]
			if found {
				f.data[idx] = flag
			}
		}
	}

	// Find in aliases.
	if !found {
		for _, alias := range flag.Aliases {
			if alias.Long != "" {
				idx, found = f.long[alias.Long]
				if found {
					f.data[idx] = flag
					break
				}
			}

			if alias.Short != "" {
				idx, found = f.short[alias.Short]
				if found {
					f.data[idx] = flag
					break
				}
			}
		}
	}

	if found {
		return
	}

	// Append a new flag.
	f.data = append(f.data, flag)
	idx = len(f.data) - 1

	if flag.Long != "" {
		if f.long == nil {
			f.long = make(map[string]int)
		}

		f.long[flag.Long] = idx
	}

	if flag.Short != "" {
		if f.short == nil {
			f.short = make(map[string]int)
		}

		f.short[flag.Short] = idx
	}

	for _, alias := range flag.Aliases {
		if alias.Long != "" {
			if f.long == nil {
				f.long = make(map[string]int)
			}

			f.long[alias.Long] = idx
		}

		if alias.Short != "" {
			if f.short == nil {
				f.short = make(map[string]int)
			}

			f.short[alias.Short] = idx
		}
	}
}

type args struct {
	data  []Arg
	index map[string]int // Indexes of named arg in the data.
}

func (a *args) Get(name string) (idx int, arg *Arg, ok bool) {
	idx, ok = a.index[name]
	if ok {
		arg = &a.data[idx]
	}
	return
}

func (a *args) Nth(i int) (arg *Arg, ok bool) {
	if i >= len(a.data) {
		return
	}

	return &a.data[i], true
}

func (a *args) Add(arg Arg) {
	if arg.Name == "" {
		return
	}

	// Find already added arg.
	idx, found := a.index[arg.Name]
	if found {
		a.data[idx] = arg
		return
	}

	// Append a new arg.
	a.data = append(a.data, arg)
	idx = len(a.data) - 1

	if a.index == nil {
		a.index = make(map[string]int)
	}

	a.index[arg.Name] = idx
}

var _ (Parser) = (*DefaultParser)(nil)

type DefaultParser struct {
	flags   flags
	args    args
	unknown []string // Unknown flags (without named flags).
	rest    []string // Other arguments (without named args).
}

func (p *DefaultParser) RegisterFlag(flag Flag) error {
	if _, _, ok := p.flags.Find(flag.Long, flag.Short, flag.Aliases); ok {
		// TODO(SuperPaintman): check if the flag already in flags.
	}

	p.flags.Add(flag)

	return nil
}

func (p *DefaultParser) RegisterArg(arg Arg) error {
	if _, _, ok := p.args.Get(arg.Name); ok {
		// TODO(SuperPaintman): check if the arg already in args.
	}

	p.args.Add(arg)

	return nil
}

func (p *DefaultParser) Parse(arguments []string) error {
	var argIdx int
	for {
		if len(arguments) == 0 {
			break
		}

		arg := arguments[0]
		arguments = arguments[1:]

		if len(arg) == 0 {
			continue
		}

		// Args.
		if arg[0] != '-' {
			a, ok := p.args.Nth(argIdx)
			if ok {
				if err := a.Value.Set(arg); err != nil {
					return err
				}
			} else {
				p.rest = append(p.rest, arg)
			}

			argIdx++

			continue
		}

		// TODO(SuperPaintman): add POSIX-style short flag combining (-a -b -> -ab).
		// TODO(SuperPaintman): add Short-flag+parameter combining (-a parm -> -aparm).
		// TODO(SuperPaintman): add required flags.
		// TODO(SuperPaintman): add optional args.

		// Flags.
		numMinuses := 1
		if arg[1] == '-' {
			numMinuses++

			// TODO(SuperPaintman): add the "--" bypass.
		}

		name := arg[numMinuses:]
		if len(name) == 0 || name[0] == '-' || name[0] == '=' {
			// return false, f.failf("bad flag syntax: %s", s)
		}

		// Find a value.
		var (
			value    string
			hasValue bool
		)
		// Equals cannot be first.
		for i := 1; i < len(name); i++ {
			if name[i] == '=' {
				value = name[i+1:]
				hasValue = true
				name = name[0:i]
				break
			}
		}

		// Find a known flag.
		_, flag, knownflag := p.flags.Get(name)

		if !hasValue && len(arguments) > 0 {
			next := arguments[0]
			if len(next) > 0 && next[0] != '-' {
				setValue := knownflag
				if knownflag {
					// Special case for bool flags. Allow only bool-like values.
					if fv, ok := flag.Value.(boolFlag); ok && fv.IsBoolFlag() {
						setValue = isBoolValue(next)
					}
				}

				if setValue {
					value = next
					hasValue = true
					arguments = arguments[1:]
				}
			}
		}

		if !knownflag {
			prefix := strings.Repeat("-", numMinuses)
			p.unknown = append(p.unknown, prefix+name)
			continue
		}

		// Set Value.
		// Special case for bool flags which doesn't need a value.
		if fv, ok := flag.Value.(boolFlag); ok && fv.IsBoolFlag() {
			if !hasValue {
				value = "true"
			}
		}

		if err := flag.Value.Set(value); err != nil {
			return err
		}
	}

	return nil
}

func (p *DefaultParser) Args() []Arg {
	return p.args.data
}

func (p *DefaultParser) Flags() []Flag {
	return p.flags.data
}

func isBoolValue(str string) bool {
	switch str {
	case "1", "t", "T", "true", "TRUE", "True",
		"0", "f", "F", "false", "FALSE", "False":
		return true
	}
	return false
}