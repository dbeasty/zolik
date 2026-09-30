import { Pressable, Text } from 'react-native';

import { t } from '@/src/lib/i18n';

/**
 * The table's join code, pinned in the bar above a board that is already dealt.
 *
 * The code is how somebody who is seated here gets back in: the join call lets
 * an existing seat through whatever the table is doing (see joinLocked on the
 * server). Before this, the only place a player could find the code was the
 * lobby, which is gone the moment the cards are dealt, so the player left at a
 * paused or swept-up table had no way to send the others back to it.
 *
 * Pressing it opens the invite-back sheet rather than copying the table link
 * outright, because the table link only works on the device a player sat down
 * with — somebody on a new one needs their seat link, which lives there too.
 */
export function TableCode({
  joinCode,
  onPress,
  chipStyle,
  textStyle,
}: {
  joinCode: string;
  onPress: () => void;
  chipStyle: object;
  textStyle: object;
}) {
  return (
    <Pressable
      testID="match-table-code"
      accessibilityRole="button"
      accessibilityLabel={`${t('match.tableCode', { code: joinCode })}, ${t('invite.backTitle')}`}
      onPress={onPress}
      hitSlop={8}
      style={chipStyle}
    >
      <Text style={textStyle}>{t('match.tableCode', { code: joinCode })}</Text>
    </Pressable>
  );
}
