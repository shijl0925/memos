import { useEffect } from "react";
import { toast } from "react-hot-toast";
import HomeSidebar from "@/components/HomeSidebar";
import MemoEditor from "@/components/MemoEditor";
import MemoFilter from "@/components/MemoFilter";
import MemoList from "@/components/MemoList";
import MobileHeader from "@/components/MobileHeader";
import { useGlobalStore, useLayoutStore, useUserStore } from "@/store/module";
import { useTranslate } from "@/utils/i18n";

const Home = () => {
  const t = useTranslate();
  const globalStore = useGlobalStore();
  const layoutStore = useLayoutStore();
  const userStore = useUserStore();
  const user = userStore.state.user;

  const openSidebar = () => {
    layoutStore.setHeaderStatus(false);
    layoutStore.setHomeSidebarStatus(true);
  };

  useEffect(() => {
    const currentUsername = userStore.getCurrentUsername();
    userStore.getUserByUsername(currentUsername).catch((error) => {
      console.error(error);
      toast.error(t("message.user-not-found"));
    });
  }, [userStore.getCurrentUsername()]);

  useEffect(() => {
    if (user?.setting.locale) {
      globalStore.setLocale(user.setting.locale);
    }
  }, [user?.setting.locale]);

  return (
    <div className="w-full flex flex-row justify-start items-start">
      <HomeSidebar />
      <div className="flex-grow min-w-0 flex justify-center pt-4">
        <div className="w-full max-w-3xl px-4">
          <MobileHeader onSearchClick={openSidebar} />
          {!userStore.isVisitorMode() && <MemoEditor className="mb-2" />}
          <MemoFilter />
          <MemoList />
        </div>
      </div>
    </div>
  );
};

export default Home;
