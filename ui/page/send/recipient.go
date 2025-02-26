package send

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/crypto-power/cryptopower/app"
	sharedW "github.com/crypto-power/cryptopower/libwallet/assets/wallet"
	libUtil "github.com/crypto-power/cryptopower/libwallet/utils"
	"github.com/crypto-power/cryptopower/ui/cryptomaterial"
	"github.com/crypto-power/cryptopower/ui/load"
	"github.com/crypto-power/cryptopower/ui/modal"
	"github.com/crypto-power/cryptopower/ui/values"
)

type recipient struct {
	*load.Load
	id int

	navigator   app.WindowNavigator
	deleteBtn   *cryptomaterial.Clickable
	description cryptomaterial.Editor

	selectedWallet        sharedW.Asset
	selectedSourceAccount *sharedW.Account

	sendDestination *destination
	amount          *sendAmount
	pageParam       getPageFields
	deleteRecipient func(int)
}

func newRecipient(l *load.Load, selectedWallet sharedW.Asset, pageParam getPageFields, id int, navigator app.WindowNavigator) *recipient {
	rp := &recipient{
		Load:           l,
		navigator:      navigator,
		selectedWallet: selectedWallet,
		pageParam:      pageParam,
		id:             id,
		deleteBtn:      l.Theme.NewClickable(false),
	}

	assetType := rp.selectedWallet.GetAssetType()

	rp.amount = newSendAmount(l.Theme, assetType)
	rp.amount.amountEditor.TextSize = values.TextSizeTransform(l.IsMobileView(), values.TextSize16)
	rp.sendDestination = newSendDestination(l, assetType)

	rp.description = rp.Theme.Editor(new(widget.Editor), values.String(values.StrNote))
	rp.description.Editor.SingleLine = false
	rp.description.Editor.SetText("")
	rp.description.IsTitleLabel = false
	// Set the maximum characters the editor can accept.
	rp.description.Editor.MaxLen = MaxTxLabelSize
	rp.description.TextSize = values.TextSizeTransform(l.IsMobileView(), values.TextSize16)
	rp.description.AlwaysShowHint()

	return rp
}

func (rp *recipient) onAddressChanged(addressChanged func()) {
	rp.sendDestination.addressChanged = addressChanged
}

func (rp *recipient) onAmountChanged(amountChanged func()) {
	rp.amount.amountChanged = amountChanged
}

func (rp *recipient) onDeleteRecipient(onDelete func(int)) {
	rp.deleteRecipient = onDelete
}

func (rp *recipient) cleanAllErrors() {
	rp.amount.setError("")
	rp.sendDestination.setError("")
}

func (rp *recipient) setDestinationAssetType(assetType libUtil.AssetType) {
	rp.amount.setAssetType(assetType)
	rp.sendDestination.initDestinationWalletSelector(assetType)
}

func (rp *recipient) isAccountValid(sourceAccount, account *sharedW.Account) bool {
	if sourceAccount == nil || account == nil {
		return false
	}
	accountIsValid := account.Number != load.MaxInt32
	// Filter mixed wallet
	destinationWallet := rp.sendDestination.walletDropdown.SelectedWallet()
	isMixedAccount := load.MixedAccountNumber(destinationWallet) == account.Number

	// Filter the sending account.
	sourceWalletID := sourceAccount.WalletID
	isSameAccount := sourceWalletID == account.WalletID && account.Number == sourceAccount.Number
	if !accountIsValid || isSameAccount || isMixedAccount {
		return false
	}
	return true
}

func (rp *recipient) initializeAccountSelectors(sourceAccount *sharedW.Account) {
	rp.selectedSourceAccount = sourceAccount
	rp.sendDestination.sourceAccount = sourceAccount
	_ = rp.sendDestination.accountDropdown.Setup(rp.sendDestination.walletDropdown.SelectedWallet())
}

func (rp *recipient) isShowSendToWallet() bool {
	sourceWalletSelected := rp.sendDestination.walletDropdown.SelectedWallet()
	if sourceWalletSelected == nil {
		return false
	}
	var wallets []sharedW.Asset
	switch sourceWalletSelected.GetAssetType() {
	case libUtil.BTCWalletAsset:
		wallets = append(wallets, rp.AssetsManager.AllBTCWallets()...)
	case libUtil.DCRWalletAsset:
		wallets = append(wallets, rp.AssetsManager.AllDCRWallets()...)
	case libUtil.LTCWalletAsset:
		wallets = append(wallets, rp.AssetsManager.AllLTCWallets()...)
	}

	if len(wallets) == 1 {
		account, err := wallets[0].GetAccountsRaw()
		if err != nil {
			log.Errorf("Error getting accounts:", err)
			return false
		}
		accountValids := make([]sharedW.Account, 0)
		for _, acc := range account.Accounts {
			if rp.isAccountValid(rp.selectedSourceAccount, acc) {
				accountValids = append(accountValids, *acc)
			}
		}
		return len(accountValids) > 0 // it should show the send to wallet if there is at least one valid account.
	}

	if len(wallets) > 1 {
		return true
	}

	return false
}

func (rp *recipient) isSendToAddress() bool {
	return rp.sendDestination.isSendToAddress()
}

func (rp *recipient) isValidated() bool {
	amountIsValid := rp.amount.amountIsValid()
	addressIsValid := rp.sendDestination.validate()
	// No need for checking the err message since it is as result of amount and
	// address validation.
	// validForSending
	return amountIsValid && addressIsValid

}

func (rp *recipient) resetFields() {
	rp.sendDestination.clearAddressInput()
	rp.description.Editor.SetText("")

	rp.amount.resetFields()
}

func (rp *recipient) destinationAddress() string {
	address, err := rp.sendDestination.destinationAddress()
	if err != nil {
		rp.addressValidationError(err.Error())
		return ""
	}
	return address
}

func (rp *recipient) destinationAccount() *sharedW.Account {
	return rp.sendDestination.destinationAccount()
}

func (rp *recipient) descriptionText() string {
	return rp.description.Editor.Text()
}

func (rp *recipient) validAmount() (int64, bool) {
	amountAtom, sendMax, err := rp.amount.validAmount()
	if err != nil {
		rp.amountValidationError(err.Error())
		return -1, false
	}

	return amountAtom, sendMax
}

func (rp *recipient) setAmount(amount int64) {
	rp.amount.setAmount(amount)
}

func (rp *recipient) amountValidationError(err string) {
	rp.amount.setError(err)
}

func (rp *recipient) addressValidationError(err string) {
	rp.sendDestination.setError(err)
}

func (rp *recipient) HandleUserInteractions(gtx C) {
	rp.sendDestination.HandleDropdownInteraction(gtx)
}

func (rp *recipient) recipientLayout(index int, showIcon bool) layout.Widget {
	return func(gtx C) D {
		rp.handle(gtx)
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.WrapContent,
			Height:      cryptomaterial.WrapContent,
			Orientation: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if showIcon {
					return layout.Inset{Bottom: values.MarginPadding12}.Layout(gtx, func(gtx C) D {
						return rp.topLayout(gtx, index)
					})
				}
				return D{}
			}),
			layout.Rigid(func(gtx C) D {
				layoutBody := func(gtx C) D {
					txt := fmt.Sprintf("%s %s", values.String(values.StrDestination), values.String(values.StrAddress))
					return rp.contentWrapper(gtx, txt, rp.sendDestination.destinationAddressEditor.Layout)
				}

				if !rp.isShowSendToWallet() {
					return layoutBody(gtx)
				}

				if !rp.isSendToAddress() {
					layoutBody = rp.walletAccountLayout()
				}

				return rp.sendDestination.accountSwitch.Layout(gtx, layoutBody, rp.IsMobileView())
			}),
			layout.Rigid(rp.addressAndAmountLayout),
			layout.Rigid(rp.txLabelSection),
		)
	}
}

func (rp *recipient) topLayout(gtx C, index int) D {
	txt := fmt.Sprintf("%s: %s %v", values.String(values.StrTo), values.String(values.StrRecipient), index)
	titleTxt := rp.Theme.Label(values.TextSizeTransform(rp.IsMobileView(), values.TextSize16), txt)
	titleTxt.Color = rp.Theme.Color.GrayText2

	return layout.Flex{}.Layout(gtx,
		layout.Rigid(titleTxt.Layout),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, func(gtx C) D {
				return rp.deleteBtn.Layout(gtx, rp.Theme.NewIcon(rp.Theme.Icons.DeleteIcon).Layout20dp)
			})
		}),
	)
}

func (rp *recipient) walletAccountLayout() layout.Widget {
	return func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:       cryptomaterial.MatchParent,
			Height:      cryptomaterial.WrapContent,
			Orientation: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				txt := fmt.Sprintf("%s %s", values.String(values.StrDestination), values.String(values.StrWallet))
				return rp.sendDestination.walletDropdown.Layout(gtx, txt)
			}),
			layout.Rigid(func(gtx C) D {
				txt := fmt.Sprintf("%s %s", values.String(values.StrDestination), values.String(values.StrAccount))
				return layout.Inset{Top: values.MarginPadding32}.Layout(gtx, func(gtx C) D {
					return rp.sendDestination.accountDropdown.Layout(gtx, txt)
				})
			}),
		)
	}
}

func (rp *recipient) contentWrapper(gtx C, title string, content layout.Widget) D {
	return layout.Inset{
		Bottom: values.MarginPadding16,
	}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				lbl := rp.Theme.Label(values.TextSizeTransform(rp.IsMobileView(), values.TextSize16), title)
				lbl.Font.Weight = font.SemiBold
				return layout.Inset{
					Bottom: values.MarginPadding4,
				}.Layout(gtx, lbl.Layout)
			}),
			layout.Rigid(content),
		)
	})
}

func (rp *recipient) addressAndAmountLayout(gtx C) D {
	widget := func(gtx C) D { return rp.amount.amountEditor.Layout(gtx) }
	if rp.pageParam().exchangeRate != -1 && rp.pageParam().usdExchangeSet {
		widget = func(gtx C) D {
			icon := cryptomaterial.NewIcon(rp.Theme.Icons.ActionSwapHoriz)
			axis := layout.Horizontal
			amountHeight := 0
			align := layout.Middle
			if !rp.IsMobileView() {
				align = layout.Start
			}
			flexChilds := []layout.FlexChild{
				layout.Flexed(0.45, func(gtx C) D {
					dims := rp.amount.amountEditor.Layout(gtx)
					amountHeight = dims.Size.Y
					return dims
				}),
				layout.Flexed(0.1, func(gtx C) D {
					if rp.IsMobileView() {
						return layout.Center.Layout(gtx, func(gtx C) D {
							return icon.Layout(gtx, values.MarginPadding16)
						})
					}
					return layout.Inset{Top: values.MarginPadding13}.Layout(gtx, func(gtx C) D {
						return icon.Layout(gtx, values.MarginPadding16)
					})
				}),
				layout.Flexed(0.45, func(gtx C) D {
					if rp.amount.amountEditor.HasError() {
						gtx.Constraints.Min.Y = amountHeight
					}
					return rp.amount.usdAmountEditor.Layout(gtx)
				}),
			}
			if rp.IsMobileView() {
				axis = layout.Vertical
				icon = cryptomaterial.NewIcon(rp.Theme.Icons.ActionSwapVertical)
				flexChilds = []layout.FlexChild{
					layout.Rigid(rp.amount.amountEditor.Layout),
					layout.Rigid(layout.Spacer{Height: values.MarginPadding10}.Layout),
					layout.Rigid(func(gtx C) D {
						return icon.Layout(gtx, values.MarginPadding16)
					}),
					layout.Rigid(layout.Spacer{Height: values.MarginPadding10}.Layout),
					layout.Rigid(rp.amount.usdAmountEditor.Layout),
				}
			}
			return layout.Flex{
				Axis:      axis,
				Alignment: align,
			}.Layout(gtx, flexChilds...)
		}

	}
	return rp.contentWrapper(gtx, values.String(values.StrAmount), widget)
}

func (rp *recipient) txLabelSection(gtx C) D {
	count := len(rp.description.Editor.Text())
	txt := fmt.Sprintf("%s (%d/%d)", values.String(values.StrDescriptionNote), count, rp.description.Editor.MaxLen)
	return rp.contentWrapper(gtx, txt, rp.description.Layout)
}

func (rp *recipient) validateAmount() {
	if len(rp.amount.amountEditor.Editor.Text()) > 0 {
		rp.amount.validateAmount()
	}
}

func (rp *recipient) restyleWidgets() {
	rp.amount.styleWidgets()
	rp.sendDestination.styleWidgets()
}

func (rp *recipient) handle(gtx C) {
	rp.sendDestination.handle(gtx)
	rp.amount.handle(gtx)

	if rp.amount.IsMaxClicked(gtx) {
		rp.amount.setError("")
		rp.amount.SendMax = true
		rp.amount.amountChanged()
	}

	if rp.deleteBtn.Clicked(gtx) {
		title := values.String(values.StrRemoveRecipient)
		msg := values.String(values.StrRemoveRecipientWarning)
		rp.showWarningModalDialog(title, msg)
	}

	// if destination switch is equal to Address
	if rp.isSendToAddress() {
		if rp.sendDestination.validate() {
			if !rp.AssetsManager.ExchangeRateFetchingEnabled() {
				if len(rp.amount.amountEditor.Editor.Text()) == 0 {
					rp.amount.SendMax = false
				}
			} else {
				if len(rp.amount.amountEditor.Editor.Text()) == 0 {
					rp.amount.usdAmountEditor.Editor.SetText("")
					rp.amount.SendMax = false
				}
			}
		}
	} else {
		if !rp.AssetsManager.ExchangeRateFetchingEnabled() {
			if len(rp.amount.amountEditor.Editor.Text()) == 0 {
				rp.amount.SendMax = false
			}
		} else {
			if len(rp.amount.amountEditor.Editor.Text()) == 0 {
				rp.amount.usdAmountEditor.Editor.SetText("")
				rp.amount.SendMax = false
			}
		}
	}
}

func (rp *recipient) showWarningModalDialog(title, msg string) {
	warningModal := modal.NewCustomModal(rp.Load).
		Title(title).
		Body(msg).
		SetNegativeButtonText(values.String(values.StrCancel)).
		SetNegativeButtonCallback(func() {

		}).
		SetNegativeButtonText(values.String(values.StrCancel)).
		PositiveButtonStyle(rp.Theme.Color.Surface, rp.Theme.Color.Danger).
		SetPositiveButtonText(values.String(values.StrRemove)).
		SetPositiveButtonCallback(func(_ bool, _ *modal.InfoModal) bool {
			rp.deleteRecipient(rp.id)
			return true
		})
	rp.navigator.ShowModal(warningModal)
}
