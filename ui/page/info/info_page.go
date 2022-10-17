package info

import (
	"context"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"

	"gitlab.com/raedah/cryptopower/app"
	"gitlab.com/raedah/cryptopower/libwallet"
	sharedW "gitlab.com/raedah/cryptopower/libwallet/assets/wallet"
	"gitlab.com/raedah/cryptopower/listeners"
	"gitlab.com/raedah/cryptopower/ui/cryptomaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	"gitlab.com/raedah/cryptopower/ui/page/seedbackup"
	"gitlab.com/raedah/cryptopower/ui/values"
	"gitlab.com/raedah/cryptopower/wallet"
)

const InfoID = "Info"

type (
	C = layout.Context
	D = layout.Dimensions
)

type WalletInfo struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	*listeners.SyncProgressListener
	*listeners.BlocksRescanProgressListener
	*listeners.TxAndBlockNotificationListener
	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	multiWallet  *libwallet.MultiWallet
	rescanUpdate *wallet.RescanUpdate

	container *widget.List

	walletStatusIcon *cryptomaterial.Icon
	syncSwitch       *cryptomaterial.Switch
	toBackup         cryptomaterial.Button
	checkBox         cryptomaterial.CheckBoxStyle

	remainingSyncTime    string
	headersToFetchOrScan int32
	stepFetchProgress    int32
	syncProgress         int
	syncStep             int

	redirectfunc seedbackup.Redirectfunc
}

func NewInfoPage(l *load.Load, redirect seedbackup.Redirectfunc) *WalletInfo {
	pg := &WalletInfo{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(InfoID),
		multiWallet:      l.WL.MultiWallet,
		container: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		checkBox: l.Theme.CheckBox(new(widget.Bool), "I am aware of the risk"),
	}

	pg.toBackup = pg.Theme.Button(values.String(values.StrBackupNow))
	pg.toBackup.Font.Weight = text.Medium
	pg.toBackup.TextSize = values.TextSize14

	pg.redirectfunc = redirect

	pg.initWalletStatusWidgets()

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *WalletInfo) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

	autoSync := pg.WL.SelectedWallet.Wallet.ReadBoolConfigValueForKey(load.AutoSyncConfigKey, false)
	pg.syncSwitch.SetChecked(autoSync)

	pg.listenForNotifications()
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
// Layout lays out the widgets for the main wallets pg.
func (pg *WalletInfo) Layout(gtx layout.Context) layout.Dimensions {
	body := func(gtx C) D {
		return pg.Theme.List(pg.container).Layout(gtx, 1, func(gtx C, i int) D {
			return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
				return pg.Theme.Card().Layout(gtx, func(gtx C) D {
					return layout.UniformInset(values.MarginPadding20).Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Inset{
									Right: values.MarginPadding10,
									Left:  values.MarginPadding10,
								}.Layout(gtx, func(gtx C) D {
									txt := pg.Theme.Body1(pg.WL.SelectedWallet.Wallet.Name)
									txt.Font.Weight = text.SemiBold
									return txt.Layout(gtx)
								})
							}),
							layout.Rigid(func(gtx C) D {
								if len(pg.WL.SelectedWallet.Wallet.EncryptedSeed) > 0 {
									return layout.Inset{
										Top: values.MarginPadding16,
									}.Layout(gtx, func(gtx C) D {
										return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
											layout.Rigid(pg.Theme.Icons.RedAlert.Layout24dp),
											layout.Rigid(func(gtx C) D {
												return layout.Inset{
													Left:  values.MarginPadding9,
													Right: values.MarginPadding16,
												}.Layout(gtx, pg.Theme.Body2(values.String(values.StrBackupWarning)).Layout)
											}),
											layout.Rigid(pg.toBackup.Layout),
										)
									})
								}
								return D{}
							}),
							layout.Rigid(pg.syncStatusSection),
						)
					})
				})
			})
		})
	}

	return components.UniformPadding(gtx, body)
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *WalletInfo) HandleUserInteractions() {
	if pg.syncSwitch.Changed() {
		if pg.WL.SelectedWallet.Wallet.IsRescanning() {
			pg.WL.SelectedWallet.Wallet.CancelRescan()
		} else {
			pg.WL.SelectedWallet.Wallet.SaveUserConfigValue(load.AutoSyncConfigKey, pg.syncSwitch.IsChecked())
			go func() {
				pg.ToggleSync()
			}()
		}
	}

	if pg.toBackup.Button.Clicked() {
		pg.ParentNavigator().Display(seedbackup.NewBackupInstructionsPage(pg.Load, pg.WL.SelectedWallet.Wallet, pg.redirectfunc))
	}
}

// listenForNotifications starts a goroutine to watch for sync updates
// and update the UI accordingly. To prevent UI lags, this method does not
// refresh the window display everytime a sync update is received. During
// active blocks sync, rescan or proposals sync, the Layout method auto
// refreshes the display every set interval. Other sync updates that affect
// the UI but occur outside of an active sync requires a display refresh.
func (pg *WalletInfo) listenForNotifications() {
	switch {
	case pg.SyncProgressListener != nil:
		return
	case pg.TxAndBlockNotificationListener != nil:
		return
	case pg.BlocksRescanProgressListener != nil:
		return
	}

	pg.SyncProgressListener = listeners.NewSyncProgress()
	err := pg.WL.SelectedWallet.Wallet.AddSyncProgressListener(pg.SyncProgressListener, InfoID)
	if err != nil {
		log.Errorf("Error adding sync progress listener: %v", err)
		return
	}

	pg.TxAndBlockNotificationListener = listeners.NewTxAndBlockNotificationListener()
	err = pg.WL.SelectedWallet.Wallet.AddTxAndBlockNotificationListener(pg.TxAndBlockNotificationListener, true, InfoID)
	if err != nil {
		log.Errorf("Error adding tx and block notification listener: %v", err)
		return
	}

	pg.BlocksRescanProgressListener = listeners.NewBlocksRescanProgressListener()
	pg.WL.SelectedWallet.Wallet.SetBlocksRescanProgressListener(pg.BlocksRescanProgressListener)

	go func() {
		for {
			select {
			case n := <-pg.SyncStatusChan:
				// Update sync progress fields which will be displayed
				// when the next UI invalidation occurs.
				switch t := n.ProgressReport.(type) {
				case *sharedW.HeadersFetchProgressReport:
					pg.stepFetchProgress = t.HeadersFetchProgress
					pg.headersToFetchOrScan = t.TotalHeadersToFetch
					pg.syncProgress = int(t.TotalSyncProgress)
					pg.remainingSyncTime = components.TimeFormat(int(t.TotalTimeRemainingSeconds), true)
					pg.syncStep = wallet.FetchHeadersSteps
				case *sharedW.AddressDiscoveryProgressReport:
					pg.syncProgress = int(t.TotalSyncProgress)
					pg.remainingSyncTime = components.TimeFormat(int(t.TotalTimeRemainingSeconds), true)
					pg.syncStep = wallet.AddressDiscoveryStep
					pg.stepFetchProgress = t.AddressDiscoveryProgress
				case *sharedW.HeadersRescanProgressReport:
					pg.headersToFetchOrScan = t.TotalHeadersToScan
					pg.syncProgress = int(t.TotalSyncProgress)
					pg.remainingSyncTime = components.TimeFormat(int(t.TotalTimeRemainingSeconds), true)
					pg.syncStep = wallet.RescanHeadersStep
					pg.stepFetchProgress = t.RescanProgress
				}

				// We only care about sync state changes here, to
				// refresh the window display.
				switch n.Stage {
				case wallet.SyncStarted:
					fallthrough
				case wallet.SyncCanceled:
					fallthrough
				case wallet.SyncCompleted:
					pg.ParentWindow().Reload()
				}

			case n := <-pg.TxAndBlockNotifChan:
				switch n.Type {
				case listeners.NewTransaction:
					pg.ParentWindow().Reload()
				case listeners.BlockAttached:
					pg.ParentWindow().Reload()
				}
			case n := <-pg.BlockRescanChan:
				pg.rescanUpdate = &n
				if n.Stage == wallet.RescanEnded {
					pg.ParentWindow().Reload()
				}
			case <-pg.ctx.Done():
				pg.WL.SelectedWallet.Wallet.RemoveSyncProgressListener(InfoID)
				pg.WL.SelectedWallet.Wallet.RemoveTxAndBlockNotificationListener(InfoID)
				pg.WL.SelectedWallet.Wallet.SetBlocksRescanProgressListener(nil)

				close(pg.SyncStatusChan)
				close(pg.TxAndBlockNotifChan)
				close(pg.BlockRescanChan)

				pg.SyncProgressListener = nil
				pg.TxAndBlockNotificationListener = nil
				pg.BlocksRescanProgressListener = nil

				return
			}
		}
	}()
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *WalletInfo) OnNavigatedFrom() {
	pg.ctxCancel()
}
