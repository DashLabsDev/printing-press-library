// pp:client-call
// Replaces generator-emitted stub: real implementation in internal/icaroclient.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRisoluzioniGetCmd(flags *rootFlags) *cobra.Command {
	var flagCommis string
	cmd := &cobra.Command{
		Use:     "get <legisl> <numero>",
		Short:   "Scarica un singolo documento da risoluzioni.",
		Example: "  ars-sicilia-pp-cli risoluzioni get 18 1 --commissione PRIMA --json",
		Args:    cobra.MaximumNArgs(2),
		Annotations: map[string]string{
			"pp:endpoint":   "risoluzioni.get",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				if dryRunOK(flags) || cliIsVerify() {
					return cmd.Help()
				}
				return usageErr(fmt.Errorf("richiesti 2 argomenti: <legisl> e <numero>"))
			}
			legisl, err := atoiArg(args[0], "legisl")
			if err != nil {
				return err
			}
			numero, err := atoiArg(args[1], "numero")
			if err != nil {
				return err
			}
			// Le risoluzioni sono numerate per commissione (1/I, 1/II…):
			// --commissione sceglie l'atto sul campo COMMIS, come --anno su
			// `leggi get`.
			var extra map[string]string
			if flagCommis != "" {
				extra = map[string]string{"commissione": flagCommis}
			}
			return runGetExtra(cmd, flags, "risoluzioni", legisl, numero, extra)
		},
	}
	cmd.Flags().StringVar(&flagCommis, "commissione", "", "Commissione, per scegliere fra atti con lo stesso numero: le risoluzioni sono numerate per commissione. Ordinale (PRIMA..SESTA) o una parola della denominazione.")
	return cmd
}
