package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/stephrobert/feint/internal/core/machine"
)

// An emulator that leaves its machines behind is worse than one that starts
// none. Everything it creates on the runtime carries a label, so this removes
// exactly its own work: an operator's own instances and bridges are never
// touched, whatever their names.
//
// It exists as a command because the situation it fixes is one a user lands in
// without doing anything wrong, a killed process being enough, and because the
// conformance suite needs the same sweep and should not reimplement it in shell.
func clean(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	vm := fs.String("vm", "incus", "machine runtime to sweep: incus, incus-vm, incus-ovn")
	if err := fs.Parse(args); err != nil {
		return err
	}

	driver, err := machineDriver(*vm, stdout)
	if err != nil {
		return err
	}
	pruner, ok := driver.(machine.Pruner)
	if !ok {
		return fmt.Errorf("the %s runtime cannot be swept", driver.Name())
	}

	pruned, err := pruner.Prune(context.Background())
	// Reported either way: a partial sweep still removed something, and saying
	// what went is what tells the operator whether to look further.
	fmt.Fprintf(stdout, "removed %d machine(s), %d network(s), %d rule set(s)\n",
		pruned.Machines, pruned.Networks, pruned.Firewalls)
	if err != nil {
		return err
	}
	if pruned.Total() == 0 {
		fmt.Fprintln(stdout, "nothing was left behind")
	}
	return nil
}
