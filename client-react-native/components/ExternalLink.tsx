import { Link } from 'expo-router';
import * as WebBrowser from 'expo-web-browser';
import type { ComponentProps } from 'react';
import { Platform } from 'react-native';

export function ExternalLink(props: Omit<ComponentProps<typeof Link>, 'href'> & { href: string }) {
  return (
    <Link
      target="_blank"
      {...props}
      // expo-router generates a union of the app's own routes for `href`, and
      // an external URL is deliberately not one of them. The assertion is the
      // point of this component: it is the single place an off-app link
      // crosses that boundary.
      href={props.href as ComponentProps<typeof Link>['href']}
      onPress={(e) => {
        if (Platform.OS !== 'web') {
          // Prevent the default behavior of linking to the default browser on native.
          e.preventDefault();
          // Open the link in an in-app browser.
          WebBrowser.openBrowserAsync(props.href);
        }
      }}
    />
  );
}
