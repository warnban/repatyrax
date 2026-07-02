using CommunityToolkit.Mvvm.Input;
using Tyrax.Core.Models;

namespace Tyrax.App.ViewModels;

/// <summary>One DOMINION invitee row.</summary>
public sealed class InviteItemViewModel
{
    public InviteItemViewModel(Invite invite, Action<string> onRemove)
    {
        InviteeId = invite.InviteeId;
        Status = invite.Status.ToUpperInvariant();
        RemoveCommand = new RelayCommand(() => onRemove(InviteeId));
    }

    public string InviteeId { get; }
    public string Status { get; }
    public IRelayCommand RemoveCommand { get; }
}
