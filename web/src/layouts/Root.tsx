import { Outlet } from "react-router-dom";
import Header from "@/components/Header";

function Root() {
  return (
    <div className="w-full h-screen overflow-hidden bg-zinc-100 dark:bg-zinc-800">
      <div className="h-full w-full flex flex-row">
        <Header />
        <main className="flex-grow min-w-0 h-full overflow-y-auto flex flex-col items-center page-wrapper">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

export default Root;
